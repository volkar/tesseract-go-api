package albums

import (
	"api/internal/domain/shared/types"
	"api/internal/platform/cache"
	"api/internal/platform/cursor"
	db "api/internal/platform/database/sqlc"
	"api/internal/platform/response"
	"context"
	_ "embed"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Repository struct {
	q             db.Querier
	cache         Cacher
	cursorManager *cursor.Cursor
	sf            singleflight.Group
}

func NewRepository(pool *pgxpool.Pool, cache Cacher, cursorManager *cursor.Cursor) *Repository {
	return &Repository{
		q:             db.New(pool),
		cache:         cache,
		cursorManager: cursorManager,
	}
}

type Cacher interface {
	Set(ctx context.Context, key string, data any) error
	Get(ctx context.Context, key string, target any) error
	Unlink(ctx context.Context, keys ...string) error
	RunScript(ctx context.Context, script *redis.Script, keys []string, args ...any) (any, error)
	Client() *redis.Client
}

type albumPaginationRow struct {
	ID     uuid.UUID
	DateAt time.Time
}

const (
	CachePrefixEntity            = "c:a:"
	CachePrefixMapper            = "c:a_m:"
	CachePrefixDirectTokenMapper = "c:a_dtm:"
)

//go:embed lua/get_album_by_mapper.lua
var getAlbumByMapperLua string
var getAlbumByMapperScript = redis.NewScript(getAlbumByMapperLua)

/* Get non deleted album by user id and album slug */
func (r *Repository) GetBySlug(ctx context.Context, userID uuid.UUID, albumSlug string) (Album, error) {
	// Get album from cache
	a, err := r.getAlbumFromCacheBySlug(ctx, userID, albumSlug)
	if err == nil {
		// Album found in cache, map and return
		return FromDB(a), nil
	}

	// Not found in cache, get album from database.
	// Use singleflight to prevent Cache Stampede
	sfKey := "sf:album:slugs:" + userID.String() + ":" + albumSlug
	val, err, _ := r.sf.Do(sfKey, func() (any, error) {
		album, dbErr := r.q.GetAlbumBySlug(ctx, db.GetAlbumBySlugParams{
			UserID:    userID,
			AlbumSlug: albumSlug,
		})
		if dbErr != nil {
			return db.Album{}, dbErr
		}

		// Async set album with mappers to cache
		bgCtx := context.WithoutCancel(ctx)
		go func(album db.Album) {
			timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
			defer cancel()
			r.setAlbumToCache(timeoutCtx, album)
		}(album)

		// Return album
		return album, nil
	})

	if err != nil {
		return Album{}, err
	}

	// Map and return
	return FromDB(val.(db.Album)), nil
}

/* Get album by user id and album id */
func (r *Repository) GetByID(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (Album, error) {
	// Get album from cache
	a, err := r.getAlbumFromCache(ctx, albumID)
	if err == nil && a.UserID == userID {
		// Album found in cache, map and return
		return FromDB(a), nil
	}

	// Not found in cache, get album from database.
	// Use singleflight to prevent Cache Stampede
	sfKey := "sf:album:id:" + albumID.String()
	val, err, _ := r.sf.Do(sfKey, func() (any, error) {
		album, dbErr := r.q.GetAlbum(ctx, db.GetAlbumParams{
			AlbumID: albumID,
			UserID:  userID,
		})
		if dbErr != nil || album.UserID != userID {
			return db.Album{}, dbErr
		}

		if !album.DeletedAt.Valid {
			// Album not deleted. Async set album with mappers to cache
			bgCtx := context.WithoutCancel(ctx)
			go func(album db.Album) {
				timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
				defer cancel()
				r.setAlbumToCache(timeoutCtx, album)
			}(album)
		}

		// Return album
		return album, nil
	})

	if err != nil {
		return Album{}, err
	}

	// Map and return
	return FromDB(val.(db.Album)), nil
}

/* Get non deleted album by direct token */
func (r *Repository) GetByDirectToken(ctx context.Context, token uuid.UUID) (Album, error) {
	// Get album from cache
	a, err := r.getAlbumFromCacheByDirectToken(ctx, token)
	if err == nil && a.IsActive && a.DirectToken.Valid && a.DirectToken.UUID == token {
		// Album found in cache, map and return
		return FromDB(a), nil
	}

	// Not found in cache, get album from database.
	// Use singleflight to prevent Cache Stampede
	sfKey := "sf:album:direct_token:" + token.String()
	nToken := uuid.NullUUID{UUID: token, Valid: true}
	val, err, _ := r.sf.Do(sfKey, func() (any, error) {
		album, dbErr := r.q.GetAlbumByDirectToken(ctx, nToken)
		if dbErr != nil {
			return db.Album{}, dbErr
		}

		// Async set album with mappers to cache
		bgCtx := context.WithoutCancel(ctx)
		go func(album db.Album) {
			timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
			defer cancel()
			r.setAlbumToCache(timeoutCtx, album)
		}(album)

		// Return album
		return album, nil
	})

	if err != nil {
		return Album{}, err
	}

	// Map and return
	return FromDB(val.(db.Album)), nil
}

/* Get paginated list of available albums by user id */
func (r *Repository) ListAvailable(ctx context.Context, userID uuid.UUID, viewerEmail string, cursor string, limit int32) ([]Album, string, error) {
	// Parse secure cursor
	cursorDate, cursorID, err := r.cursorManager.Parse(cursor)
	if err != nil {
		return []Album{}, "", response.ErrInvalidCursor.Wrap(err)
	}

	// Fetch list of IDs from database
	fetchLimit := limit + 1
	dbRows, err := r.q.ListAvailableAlbumIDs(ctx, db.ListAvailableAlbumIDsParams{
		UserID:       userID,
		ViewerEmail:  viewerEmail,
		CursorDateAt: pgtype.Timestamptz{Time: cursorDate, Valid: cursor != ""},
		CursorID:     cursorID,
		Limit:        fetchLimit,
	})
	if err != nil {
		return []Album{}, "", err
	}

	// Map sqlc specific type to our common pagination row
	idRows := make([]albumPaginationRow, len(dbRows))
	for i, row := range dbRows {
		idRows[i] = albumPaginationRow{
			ID:     row.ID,
			DateAt: row.DateAt,
		}
	}

	return r.hydrateAlbumsList(ctx, idRows, limit)
}

/* Get paginated list of owned albums by user id */
func (r *Repository) ListOwned(ctx context.Context, userID uuid.UUID, cursor string, limit int32) ([]Album, string, error) {
	// Parse secure cursor
	cursorDate, cursorID, err := r.cursorManager.Parse(cursor)
	if err != nil {
		return []Album{}, "", response.ErrInvalidCursor.Wrap(err)
	}

	// Fetch list of IDs from database
	fetchLimit := limit + 1
	dbRows, err := r.q.ListOwnedAlbumIDs(ctx, db.ListOwnedAlbumIDsParams{
		UserID:       userID,
		CursorDateAt: pgtype.Timestamptz{Time: cursorDate, Valid: cursor != ""},
		CursorID:     cursorID,
		Limit:        fetchLimit,
	})
	if err != nil {
		return []Album{}, "", err
	}

	// Map sqlc specific type to our common pagination row
	idRows := make([]albumPaginationRow, len(dbRows))
	for i, row := range dbRows {
		idRows[i] = albumPaginationRow{
			ID:     row.ID,
			DateAt: row.DateAt,
		}
	}

	return r.hydrateAlbumsList(ctx, idRows, limit)
}

/* Get paginated list of trashed albums by user id */
func (r *Repository) ListTrashed(ctx context.Context, userID uuid.UUID, cursor string, limit int32) ([]Album, string, error) {
	// Parse secure cursor
	cursorDate, cursorID, err := r.cursorManager.Parse(cursor)
	if err != nil {
		return []Album{}, "", response.ErrInvalidCursor.Wrap(err)
	}

	// Fetch list of IDs from database
	fetchLimit := limit + 1
	albums, err := r.q.ListTrashedAlbums(ctx, db.ListTrashedAlbumsParams{
		UserID:          userID,
		CursorDeletedAt: pgtype.Timestamptz{Time: cursorDate, Valid: cursor != ""},
		CursorID:        cursorID,
		Limit:           fetchLimit,
	})
	if err != nil {
		return []Album{}, "", err
	}

	// Calculate next cursor if needed
	var nextCursor string
	if len(albums) == int(fetchLimit) {
		albums = albums[:limit]
		lastItem := albums[len(albums)-1]
		nextCursor, _ = r.cursorManager.Encode(lastItem.DeletedAt.Time, lastItem.ID.String())
	}

	// Map albums
	mappedAlbums := FromDBList(albums)

	return mappedAlbums, nextCursor, nil
}

/* Create album */
func (r *Repository) Create(ctx context.Context, title string, slug string, cover string, atlas types.Atlas, access types.Access, sharedEmails []string, isActive bool, dateAt time.Time, userID uuid.UUID) (Album, error) {
	a, err := r.q.CreateAlbum(ctx, db.CreateAlbumParams{
		UserID:       userID,
		Title:        title,
		Slug:         slug,
		Cover:        cover,
		Atlas:        atlas,
		Access:       access,
		IsActive:     isActive,
		SharedEmails: sharedEmails,
		DateAt:       dateAt,
	})
	if err != nil {
		return Album{}, err
	}

	// Async set album with mappers to cache
	bgCtx := context.WithoutCancel(ctx)
	go func(album db.Album) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.setAlbumToCache(timeoutCtx, album)
	}(a)

	// Map and return
	return FromDB(a), nil
}

/* Update album */
func (r *Repository) Update(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, title string, slug string, cover string, atlas types.Atlas, access types.Access, sharedEmails []string, dateAt time.Time, isActive bool) (Album, error) {
	a, err := r.q.UpdateAlbum(ctx, db.UpdateAlbumParams{
		AlbumID:      albumID,
		UserID:       userID,
		Title:        title,
		Slug:         slug,
		Cover:        cover,
		Atlas:        atlas,
		Access:       access,
		SharedEmails: sharedEmails,
		DateAt:       dateAt,
		IsActive:     isActive,
	})
	if err != nil {
		return Album{}, err
	}

	// Async update album with mapper in cache
	bgCtx := context.WithoutCancel(ctx)
	go func(album db.Album, oldSlug string) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		// Invalidate album mapper cache (album slug changed)
		if oldSlug != album.Slug {
			r.invalidateAlbumMapperCacheOnly(timeoutCtx, album.UserID, oldSlug)
		}
		// Set new album with mapper to cache
		r.setAlbumToCache(timeoutCtx, album)
	}(a.Album, a.OldSlug)

	// Map and return
	return FromDB(a.Album), err
}

/* Update direct token */
func (r *Repository) UpdateDirectToken(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, token uuid.NullUUID) (uuid.NullUUID, error) {
	a, err := r.q.UpdateAlbumDirectToken(ctx, db.UpdateAlbumDirectTokenParams{
		AlbumID:     albumID,
		UserID:      userID,
		DirectToken: token,
	})
	if err != nil {
		return uuid.NullUUID{}, err
	}

	// We need to update the cache synchronously because the direct token is used to fetch the album
	if a.OldDirectToken.Valid {
		r.invalidateAlbumDirectTokenCacheOnly(ctx, a.OldDirectToken)
	}
	// Set new album with mapper to cache
	r.setAlbumToCache(ctx, a.Album)

	// Map and return
	return a.Album.DirectToken, err
}

/* Update album active state */
func (r *Repository) ToggleActive(ctx context.Context, userID uuid.UUID, albumID uuid.UUID, isActive bool) (uuid.UUID, error) {
	a, err := r.q.ToggleAlbumActive(ctx, db.ToggleAlbumActiveParams{
		AlbumID:  albumID,
		UserID:   userID,
		IsActive: isActive,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	// Set new album with mapper to cache
	r.setAlbumToCache(ctx, a.Album)

	return a.Album.ID, err
}

/* Delete album */
func (r *Repository) Delete(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	a, err := r.q.SoftDeleteAlbum(ctx, db.SoftDeleteAlbumParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return uuid.Nil, err
	}

	// Async invalidate album cache (entity + mapper)
	bgCtx := context.WithoutCancel(ctx)
	go func(album db.Album) {
		timeoutCtx, cancel := context.WithTimeout(bgCtx, 100*time.Millisecond)
		defer cancel()
		r.invalidateAlbumCache(timeoutCtx, album)
	}(a.Album)

	return a.Album.ID, nil
}

/* Restore deleted album */
func (r *Repository) Restore(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	id, err := r.q.RestoreAlbum(ctx, db.RestoreAlbumParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

/* Purge deleted album */
func (r *Repository) Purge(ctx context.Context, userID uuid.UUID, albumID uuid.UUID) (uuid.UUID, error) {
	id, err := r.q.HardDeleteAlbum(ctx, db.HardDeleteAlbumParams{
		AlbumID: albumID,
		UserID:  userID,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	return id, nil
}

func (r *Repository) hydrateAlbumsList(ctx context.Context, idRows []albumPaginationRow, limit int32) ([]Album, string, error) {
	if len(idRows) == 0 {
		return []Album{}, "", nil
	}

	fetchLimit := limit + 1

	// Calculate next cursor if needed
	var nextCursor string
	if len(idRows) == int(fetchLimit) {
		idRows = idRows[:limit]
		lastItem := idRows[len(idRows)-1]
		nextCursor, _ = r.cursorManager.Encode(lastItem.DateAt, lastItem.ID.String())
	}

	// Get albums from cache
	var cacheKeys []string
	for _, row := range idRows {
		cacheKeys = append(cacheKeys, CachePrefixEntity+row.ID.String())
	}

	albumsPtrs, _ := cache.MGetItems[db.Album](ctx, r.cache.Client(), cacheKeys)

	albumsArray := make(map[uuid.UUID]db.Album, len(idRows))
	for i, albumPtr := range albumsPtrs {
		if albumPtr != nil {
			albumsArray[idRows[i].ID] = *albumPtr
		}
	}

	var missingIDs []uuid.UUID

	// Find missing albums
	for _, row := range idRows {
		if albumsArray[row.ID].ID == uuid.Nil {
			missingIDs = append(missingIDs, row.ID)
		}
	}

	// Process missing albums
	if len(missingIDs) > 0 {
		dbAlbums, err := r.q.GetAlbumsByIDs(ctx, missingIDs)
		if err != nil {
			return []Album{}, "", err
		}

		// Detach context from parent
		bgCtx := context.WithoutCancel(ctx)

		for _, dbAlbum := range dbAlbums {
			albumsArray[dbAlbum.ID] = dbAlbum
			// Async set missing albums to cache
			go func(a db.Album, bCtx context.Context) {
				timeoutCtx, cancel := context.WithTimeout(bCtx, 100*time.Millisecond)
				defer cancel()
				r.setAlbumToCache(timeoutCtx, a)
			}(dbAlbum, bgCtx)
		}
	}

	// Restore strict ordering
	finalAlbums := make([]Album, 0, len(idRows))
	for _, row := range idRows {
		if album, ok := albumsArray[row.ID]; ok {
			finalAlbums = append(finalAlbums, FromDB(album))
		}
	}

	return finalAlbums, nextCursor, nil
}

/* Get album from cache by user id and album slug */
func (r *Repository) getAlbumFromCacheBySlug(ctx context.Context, userID uuid.UUID, albumSlug string) (db.Album, error) {
	res, err := r.cache.RunScript(ctx, getAlbumByMapperScript,
		[]string{CachePrefixMapper + userID.String() + ":" + albumSlug},
		CachePrefixEntity,
	)
	if err != nil || res == nil {
		return db.Album{}, err
	}

	var album db.Album
	if err := json.Unmarshal([]byte(res.(string)), &album); err != nil {
		return db.Album{}, err
	}

	return album, nil
}

/* Get album from cache by album id */
func (r *Repository) getAlbumFromCache(ctx context.Context, id uuid.UUID) (db.Album, error) {
	var a db.Album
	err := r.cache.Get(ctx, CachePrefixEntity+id.String(), &a)

	if err != nil {
		return db.Album{}, err
	}

	return a, nil
}

/* Get album from cache by direct token */
func (r *Repository) getAlbumFromCacheByDirectToken(ctx context.Context, token uuid.UUID) (db.Album, error) {
	res, err := r.cache.RunScript(ctx, getAlbumByMapperScript,
		[]string{CachePrefixDirectTokenMapper + token.String()},
		CachePrefixEntity,
	)
	if err != nil || res == nil {
		return db.Album{}, err
	}

	var album db.Album
	if err := json.Unmarshal([]byte(res.(string)), &album); err != nil {
		return db.Album{}, err
	}

	return album, nil
}

/* Set album to cache */
func (r *Repository) setAlbumToCache(ctx context.Context, album db.Album) {
	r.cache.Set(ctx, CachePrefixEntity+album.ID.String(), album)
	r.cache.Set(ctx, CachePrefixMapper+album.UserID.String()+":"+album.Slug, album.ID.String())
	if album.DirectToken.Valid {
		r.cache.Set(ctx, CachePrefixDirectTokenMapper+album.DirectToken.UUID.String(), album.ID.String())
	}
}

/* Invalidate album cache (entity + mapper) */
func (r *Repository) invalidateAlbumCache(ctx context.Context, album db.Album) {
	r.cache.Unlink(ctx, CachePrefixEntity+album.ID.String(), CachePrefixMapper+album.UserID.String()+":"+album.Slug)
	if album.DirectToken.Valid {
		r.cache.Unlink(ctx, CachePrefixDirectTokenMapper+album.DirectToken.UUID.String())
	}
}

/* Invalidate album mapper cache */
func (r *Repository) invalidateAlbumMapperCacheOnly(ctx context.Context, userID uuid.UUID, slug string) {
	r.cache.Unlink(ctx, CachePrefixMapper+userID.String()+":"+slug)
}

/* Invalidate album direct token cache mapper */
func (r *Repository) invalidateAlbumDirectTokenCacheOnly(ctx context.Context, token uuid.NullUUID) {
	r.cache.Unlink(ctx, CachePrefixDirectTokenMapper+token.UUID.String())
}
