// Package musicserver 提供音乐流媒体服务功能
package musicserver

import (
	"time"
)

// Song 歌曲.
type Song struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Artist      string     `json:"artist"`
	Album       string     `json:"album"`
	AlbumArtist string     `json:"album_artist"`
	Genre       string     `json:"genre"`
	Year        int        `json:"year"`
	Track       int        `json:"track"`
	Disc        int        `json:"disc"`
	Duration    int        `json:"duration"` // 秒
	Format      string     `json:"format"`   // mp3, flac, aac, ogg, wav
	Bitrate     int        `json:"bitrate"`
	SampleRate  int        `json:"sample_rate"`
	FileSize    int64      `json:"file_size"`
	FilePath    string     `json:"file_path"`
	CoverArtID  string     `json:"cover_art_id,omitempty"`
	LyricsPath  string     `json:"lyrics_path,omitempty"`
	Owner       string     `json:"owner"`
	PlayCount   int        `json:"play_count"`
	LastPlayed  *time.Time `json:"last_played,omitempty"`
	IsFavorite  bool       `json:"is_favorite"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Album 专辑.
type Album struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Artist        string    `json:"artist"`
	AlbumArtist   string    `json:"album_artist"`
	Genre         string    `json:"genre"`
	Year          int       `json:"year"`
	CoverArtID    string    `json:"cover_art_id,omitempty"`
	SongCount     int       `json:"song_count"`
	TotalDuration int       `json:"total_duration"`
	Owner         string    `json:"owner"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Artist 艺术家.
type Artist struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	AlbumCount int       `json:"album_count"`
	SongCount  int       `json:"song_count"`
	Owner      string    `json:"owner"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Playlist 播放列表.
type Playlist struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Owner         string    `json:"owner"`
	IsPublic      bool      `json:"is_public"`
	SongCount     int       `json:"song_count"`
	TotalDuration int       `json:"total_duration"`
	CoverArtID    string    `json:"cover_art_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PlaylistSong 播放列表歌曲.
type PlaylistSong struct {
	SongID   string    `json:"song_id"`
	Position int       `json:"position"`
	AddedAt  time.Time `json:"added_at"`
	AddedBy  string    `json:"added_by"`
}

// PlayQueue 播放队列.
type PlayQueue struct {
	ID           string          `json:"id"`
	Owner        string          `json:"owner"`
	Songs        []PlayQueueSong `json:"songs"`
	CurrentIndex int             `json:"current_index"`
	Shuffle      bool            `json:"shuffle"`
	Repeat       RepeatMode      `json:"repeat"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// PlayQueueSong 播放队列歌曲.
type PlayQueueSong struct {
	SongID   string `json:"song_id"`
	Position int    `json:"position"`
}

// RepeatMode 重复模式.
type RepeatMode string

const (
	RepeatOff RepeatMode = "off"
	RepeatAll RepeatMode = "all"
	RepeatOne RepeatMode = "one"
)

// CoverArt 专辑封面.
type CoverArt struct {
	ID        string    `json:"id"`
	SongID    string    `json:"song_id,omitempty"`
	AlbumID   string    `json:"album_id,omitempty"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	FilePath  string    `json:"file_path"`
	CreatedAt time.Time `json:"created_at"`
}

// Lyrics 歌词.
type Lyrics struct {
	SongID  string      `json:"song_id"`
	Format  string      `json:"format"` // lrc, txt
	Content string      `json:"content"`
	Lines   []LyricLine `json:"lines,omitempty"`
}

// LyricLine 歌词行.
type LyricLine struct {
	Time time.Duration `json:"time"`
	Text string        `json:"text"`
}

// PlayStats 播放统计.
type PlayStats struct {
	TotalPlays     int          `json:"total_plays"`
	TotalSongs     int          `json:"total_songs"`
	TotalAlbums    int          `json:"total_albums"`
	TotalArtists   int          `json:"total_artists"`
	TopSongs       []SongStat   `json:"top_songs,omitempty"`
	TopArtists     []ArtistStat `json:"top_artists,omitempty"`
	RecentlyPlayed []*Song      `json:"recently_played,omitempty"`
}

// SongStat 歌曲统计.
type SongStat struct {
	SongID    string `json:"song_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	PlayCount int    `json:"play_count"`
}

// ArtistStat 艺术家统计.
type ArtistStat struct {
	ArtistID  string `json:"artist_id"`
	Name      string `json:"name"`
	PlayCount int    `json:"play_count"`
}

// ========== 请求/响应结构 ==========

// CreatePlaylistRequest 创建播放列表请求.
type CreatePlaylistRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
	Owner       string `json:"owner"`
	IsPublic    bool   `json:"is_public"`
}

// UpdatePlaylistRequest 更新播放列表请求.
type UpdatePlaylistRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// AddSongToPlaylistRequest 添加歌曲到播放列表请求.
type AddSongToPlaylistRequest struct {
	SongID   string `json:"song_id" binding:"required"`
	Position *int   `json:"position,omitempty"`
}

// UpdatePlayQueueRequest 更新播放队列请求.
type UpdatePlayQueueRequest struct {
	SongIDs      []string    `json:"song_ids"`
	CurrentIndex *int        `json:"current_index,omitempty"`
	Shuffle      *bool       `json:"shuffle,omitempty"`
	Repeat       *RepeatMode `json:"repeat,omitempty"`
}

// ScanLibraryRequest 扫描音乐库请求.
type ScanLibraryRequest struct {
	Path  string `json:"path" binding:"required"`
	Owner string `json:"owner"`
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Query string `form:"q" binding:"required"`
	Type  string `form:"type"` // song, album, artist, all
	Owner string `form:"owner"`
}

// FavoriteRequest 收藏请求.
type FavoriteRequest struct {
	IsFavorite bool `json:"is_favorite"`
}

// ImportPlaylistRequest 导入播放列表请求.
type ImportPlaylistRequest struct {
	Name     string   `json:"name" binding:"required"`
	SongIDs  []string `json:"song_ids" binding:"required"`
	Owner    string   `json:"owner"`
	IsPublic bool     `json:"is_public"`
}

// ========== Subsonic API 结构 ==========

// SubsonicResponse Subsonic API 响应.
type SubsonicResponse struct {
	XMLName       interface{}           `xml:"subsonic-response" json:"-"`
	Status        string                `xml:"status,attr" json:"status"`
	Version       string                `xml:"version,attr" json:"version"`
	Type          string                `xml:"type,attr" json:"type"`
	ServerVersion string                `xml:"serverVersion,attr" json:"serverVersion"`
	OpenSubsonic  bool                  `xml:"openSubsonic,attr" json:"openSubsonic"`
	Error         *SubsonicError        `xml:"error,omitempty" json:"error,omitempty"`
	Songs         []SubsonicSong        `xml:"song,omitempty" json:"song,omitempty"`
	Albums        []SubsonicAlbum       `xml:"album,omitempty" json:"album,omitempty"`
	Artists       []SubsonicArtist      `xml:"artist,omitempty" json:"artist,omitempty"`
	Playlists     []SubsonicPlaylist    `xml:"playlists>playlist,omitempty" json:"playlists,omitempty"`
	SearchResult  *SubsonicSearchResult `xml:"searchResult2,omitempty" json:"searchResult2,omitempty"`
	NowPlaying    []SubsonicNowPlaying  `xml:"nowPlaying>entry,omitempty" json:"nowPlaying,omitempty"`
}

// SubsonicError Subsonic API 错误.
type SubsonicError struct {
	Code    int    `xml:"code,attr" json:"code"`
	Message string `xml:"message,attr" json:"message"`
}

// SubsonicSong Subsonic API 歌曲.
type SubsonicSong struct {
	ID          string `xml:"id,attr" json:"id"`
	Title       string `xml:"title,attr" json:"title"`
	Artist      string `xml:"artist,attr" json:"artist"`
	Album       string `xml:"album,attr" json:"album"`
	Genre       string `xml:"genre,attr" json:"genre"`
	Year        int    `xml:"year,attr" json:"year"`
	Track       int    `xml:"track,attr" json:"track"`
	Duration    int    `xml:"duration,attr" json:"duration"`
	ContentType string `xml:"contentType,attr" json:"contentType"`
	Suffix      string `xml:"suffix,attr" json:"suffix"`
	BitRate     int    `xml:"bitRate,attr" json:"bitRate"`
	Path        string `xml:"path,attr" json:"path"`
	PlayCount   int    `xml:"playCount,attr" json:"playCount"`
	CoverArt    string `xml:"coverArt,attr" json:"coverArt"`
}

// SubsonicAlbum Subsonic API 专辑.
type SubsonicAlbum struct {
	ID        string `xml:"id,attr" json:"id"`
	Name      string `xml:"name,attr" json:"name"`
	Artist    string `xml:"artist,attr" json:"artist"`
	Genre     string `xml:"genre,attr" json:"genre"`
	Year      int    `xml:"year,attr" json:"year"`
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	CoverArt  string `xml:"coverArt,attr" json:"coverArt"`
}

// SubsonicArtist Subsonic API 艺术家.
type SubsonicArtist struct {
	ID         string `xml:"id,attr" json:"id"`
	Name       string `xml:"name,attr" json:"name"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
}

// SubsonicPlaylist Subsonic API 播放列表.
type SubsonicPlaylist struct {
	ID        string `xml:"id,attr" json:"id"`
	Name      string `xml:"name,attr" json:"name"`
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	Owner     string `xml:"owner,attr" json:"owner"`
	Public    bool   `xml:"public,attr" json:"public"`
}

// SubsonicSearchResult Subsonic API 搜索结果.
type SubsonicSearchResult struct {
	Songs   []SubsonicSong   `xml:"song,omitempty" json:"song,omitempty"`
	Albums  []SubsonicAlbum  `xml:"album,omitempty" json:"album,omitempty"`
	Artists []SubsonicArtist `xml:"artist,omitempty" json:"artist,omitempty"`
}

// SubsonicNowPlaying Subsonic API 正在播放.
type SubsonicNowPlaying struct {
	SubsonicSong
	Username   string `xml:"username,attr" json:"username"`
	MinutesAgo int    `xml:"minutesAgo,attr" json:"minutesAgo"`
}
