package aimusicstudio

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// MusicGenre 音乐流派
type MusicGenre string

const (
	GenrePop     MusicGenre = "pop"
	GenreRock    MusicGenre = "rock"
	GenreJazz    MusicGenre = "jazz"
	GenreClassical MusicGenre = "classical"
	GenreElectronic MusicGenre = "electronic"
	GenreHipHop  MusicGenre = "hiphop"
	GenreRnB     MusicGenre = "rnb"
	GenreCountry MusicGenre = "country"
)

// Mood 情绪标签
type Mood string

const (
	MoodHappy    Mood = "happy"
	MoodSad      Mood = "sad"
	MoodEnergetic Mood = "energetic"
	MoodCalm     Mood = "calm"
	MoodRomantic Mood = "romantic"
	MoodDark     Mood = "dark"
)

// Track 音轨
type Track struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Instrument string   `json:"instrument"`
	Notes     []Note    `json:"notes"`
	Volume    float64   `json:"volume"`
	Pan       float64   `json:"pan"` // -1.0 to 1.0
	Muted     bool      `json:"muted"`
	Solo      bool      `json:"solo"`
	CreatedAt time.Time `json:"created_at"`
}

// Note 音符
type Note struct {
	Pitch     int     `json:"pitch"`     // MIDI note number (0-127)
	Duration  float64 `json:"duration"`  // in beats
	Velocity  int     `json:"velocity"`  // 0-127
	StartTime float64 `json:"start_time"` // in beats from start
}

// Composition 作品
type Composition struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Genre       MusicGenre `json:"genre"`
	Mood        Mood      `json:"mood"`
	BPM         int       `json:"bpm"`
	Key         string    `json:"key"`
	TimeSignature string  `json:"time_signature"`
	Tracks      []Track   `json:"tracks"`
	Duration    float64   `json:"duration"` // in seconds
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags"`
}

// MixConfig 混音配置
type MixConfig struct {
	CompositionID string             `json:"composition_id"`
	TrackMixes    map[string]TrackMix `json:"track_mixes"`
	MasterVolume  float64            `json:"master_volume"`
	MasterEQ      EQConfig           `json:"master_eq"`
	Compressor    CompressorConfig   `json:"compressor"`
	Reverb        ReverbConfig       `json:"reverb"`
}

// TrackMix 单轨混音
type TrackMix struct {
	Volume    float64 `json:"volume"`
	Pan       float64 `json:"pan"`
	EQ        EQConfig `json:"eq"`
	Compressor CompressorConfig `json:"compressor"`
	Delay     DelayConfig `json:"delay"`
}

// EQConfig 均衡器配置
type EQConfig struct {
	LowGain   float64 `json:"low_gain"`   // dB
	MidGain   float64 `json:"mid_gain"`   // dB
	HighGain  float64 `json:"high_gain"`  // dB
	LowFreq   float64 `json:"low_freq"`
	MidFreq   float64 `json:"mid_freq"`
	HighFreq  float64 `json:"high_freq"`
}

// CompressorConfig 压缩器配置
type CompressorConfig struct {
	Threshold float64 `json:"threshold"` // dB
	Ratio     float64 `json:"ratio"`
	Attack    float64 `json:"attack"`    // ms
	Release   float64 `json:"release"`   // ms
	Gain      float64 `json:"gain"`      // dB
}

// ReverbConfig 混响配置
type ReverbConfig struct {
	RoomSize float64 `json:"room_size"` // 0.0-1.0
	Damping  float64 `json:"damping"`   // 0.0-1.0
	Wet      float64 `json:"wet"`       // 0.0-1.0
	Dry      float64 `json:"dry"`       // 0.0-1.0
}

// DelayConfig 延迟配置
type DelayConfig struct {
	Time    float64 `json:"time"`    // ms
	Feedback float64 `json:"feedback"` // 0.0-1.0
	Mix     float64 `json:"mix"`     // 0.0-1.0
}

// GenerateRequest AI作曲请求
type GenerateRequest struct {
	Genre       MusicGenre `json:"genre"`
	Mood        Mood       `json:"mood"`
	Duration    float64    `json:"duration"` // seconds
	BPM         int        `json:"bpm"`
	Key         string     `json:"key"`
	Instruments []string   `json:"instruments"`
	Tags        []string   `json:"tags"`
}

// Service 音乐工作室服务
type Service struct {
	compositions map[string]*Composition
	mixes        map[string]*MixConfig
	mu           sync.RWMutex
}

// NewService 创建服务
func NewService() *Service {
	return &Service{
		compositions: make(map[string]*Composition),
		mixes:        make(map[string]*MixConfig),
	}
}

// GenerateComposition AI生成作品
func (s *Service) GenerateComposition(ctx context.Context, req GenerateRequest) (*Composition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	comp := &Composition{
		ID:            fmt.Sprintf("comp_%d", time.Now().UnixNano()),
		Title:         s.generateTitle(req.Genre, req.Mood),
		Genre:         req.Genre,
		Mood:          req.Mood,
		BPM:           req.BPM,
		Key:           req.Key,
		TimeSignature: "4/4",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Tags:          req.Tags,
	}

	if comp.BPM == 0 {
		comp.BPM = s.defaultBPM(req.Genre)
	}
	if comp.Key == "" {
		comp.Key = s.defaultKey(req.Mood)
	}

	// 生成音轨
	for _, inst := range req.Instruments {
		track := s.generateTrack(inst, req.Genre, req.Mood, comp.BPM, req.Duration)
		comp.Tracks = append(comp.Tracks, track)
	}

	// 如果没有指定乐器，生成默认音轨
	if len(comp.Tracks) == 0 {
		comp.Tracks = s.generateDefaultTracks(req.Genre, req.Mood, comp.BPM, req.Duration)
	}

	comp.Duration = req.Duration
	s.compositions[comp.ID] = comp

	return comp, nil
}

// generateTrack 生成音轨
func (s *Service) generateTrack(instrument string, genre MusicGenre, mood Mood, bpm int, duration float64) Track {
	track := Track{
		ID:         fmt.Sprintf("track_%d", time.Now().UnixNano()),
		Name:       instrument,
		Instrument: instrument,
		Volume:     0.8,
		Pan:        0.0,
		CreatedAt:  time.Now(),
	}

	// 根据乐器和风格生成音符
	beatsPerSec := float64(bpm) / 60.0
	totalBeats := duration * beatsPerSec

	scale := s.getScale(s.defaultKey(mood))
	noteCount := int(totalBeats / 0.5) // 每半拍一个音符

	for i := 0; i < noteCount; i++ {
		note := Note{
			Pitch:     scale[rand.Intn(len(scale))],
			Duration:  0.5,
			Velocity:  80 + rand.Intn(47), // 80-127
			StartTime: float64(i) * 0.5,
		}
		track.Notes = append(track.Notes, note)
	}

	return track
}

// generateDefaultTracks 生成默认音轨组
func (s *Service) generateDefaultTracks(genre MusicGenre, mood Mood, bpm int, duration float64) []Track {
	instruments := s.getDefaultInstruments(genre)
	var tracks []Track

	for _, inst := range instruments {
		track := s.generateTrack(inst, genre, mood, bpm, duration)
		tracks = append(tracks, track)
	}

	return tracks
}

// getDefaultInstruments 获取默认乐器组
func (s *Service) getDefaultInstruments(genre MusicGenre) []string {
	switch genre {
	case GenreRock:
		return []string{"electric_guitar", "bass", "drums", "keyboard"}
	case GenreJazz:
		return []string{"piano", "saxophone", "upright_bass", "drums"}
	case GenreClassical:
		return []string{"violin", "viola", "cello", "flute"}
	case GenreElectronic:
		return []string{"synth_lead", "synth_pad", "bass_synth", "drum_machine"}
	case GenreHipHop:
		return []string{"drum_machine", "bass_synth", "synth_lead", "sampler"}
	case GenreRnB:
		return []string{"piano", "bass", "drums", "synth_pad"}
	default:
		return []string{"piano", "guitar", "bass", "drums"}
	}
}

// getScale 获取音阶
func (s *Service) getScale(key string) []int {
	// C大调音阶的MIDI音符
	baseNotes := map[string]int{
		"C": 60, "D": 62, "E": 64, "F": 65, "G": 67, "A": 69, "B": 71,
	}

	base, ok := baseNotes[key]
	if !ok {
		base = 60 // 默认C
	}

	// 大调音阶：全全半全全全半
	scale := []int{0, 2, 4, 5, 7, 9, 11, 12}
	var notes []int
	for _, interval := range scale {
		notes = append(notes, base+interval)
	}

	return notes
}

// defaultBPM 获取默认BPM
func (s *Service) defaultBPM(genre MusicGenre) int {
	switch genre {
	case GenrePop:
		return 120
	case GenreRock:
		return 140
	case GenreJazz:
		return 100
	case GenreClassical:
		return 80
	case GenreElectronic:
		return 128
	case GenreHipHop:
		return 90
	case GenreRnB:
		return 95
	default:
		return 120
	}
}

// defaultKey 获取默认调性
func (s *Service) defaultKey(mood Mood) string {
	switch mood {
	case MoodHappy:
		return "C"
	case MoodSad:
		return "A"
	case MoodEnergetic:
		return "G"
	case MoodCalm:
		return "F"
	case MoodRomantic:
		return "D"
	case MoodDark:
		return "E"
	default:
		return "C"
	}
}

// generateTitle 生成标题
func (s *Service) generateTitle(genre MusicGenre, mood Mood) string {
	adjectives := map[Mood][]string{
		MoodHappy:    {"Sunny", "Bright", "Joyful", "Cheerful"},
		MoodSad:      {"Melancholy", "Blue", "Sorrowful", "Tearful"},
		MoodEnergetic: {"Electric", "Dynamic", "Powerful", "Blazing"},
		MoodCalm:     {"Peaceful", "Serene", "Tranquil", "Gentle"},
		MoodRomantic: {"Romantic", "Tender", "Passionate", "Dreamy"},
		MoodDark:     {"Dark", "Mysterious", "Shadowy", "Haunting"},
	}

	nouns := map[MusicGenre][]string{
		GenrePop:     {"Hit", "Single", "Chart", "Melody"},
		GenreRock:    {"Anthem", "Riff", "Storm", "Thunder"},
		GenreJazz:    {"Groove", "Swing", "Blue Note", "Standard"},
		GenreClassical: {"Sonata", "Symphony", "Concerto", "Nocturne"},
		GenreElectronic: {"Beat", "Drop", "Wave", "Pulse"},
		GenreHipHop:  {"Flow", "Beat", "Rhyme", "Verse"},
		GenreRnB:     {"Soul", "Vibe", "Rhythm", "Groove"},
	}

	adj := adjectives[mood]
	noun := nouns[genre]

	if len(adj) == 0 {
		adj = []string{"Beautiful", "Amazing", "Wonderful"}
	}
	if len(noun) == 0 {
		noun = []string{"Song", "Track", "Piece"}
	}

	return fmt.Sprintf("%s %s", adj[rand.Intn(len(adj))], noun[rand.Intn(len(noun))])
}

// GetComposition 获取作品
func (s *Service) GetComposition(id string) (*Composition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comp, ok := s.compositions[id]
	if !ok {
		return nil, fmt.Errorf("composition not found: %s", id)
	}

	return comp, nil
}

// ListCompositions 列出作品
func (s *Service) ListCompositions(genre MusicGenre, mood Mood) []*Composition {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Composition
	for _, comp := range s.compositions {
		if (genre == "" || comp.Genre == genre) &&
			(mood == "" || comp.Mood == mood) {
			result = append(result, comp)
		}
	}

	return result
}

// UpdateMix 更新混音
func (s *Service) UpdateMix(mix *MixConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mixes[mix.CompositionID] = mix
	return nil
}

// GetMix 获取混音
func (s *Service) GetMix(compositionID string) (*MixConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mix, ok := s.mixes[compositionID]
	if !ok {
		return nil, fmt.Errorf("mix not found for composition: %s", compositionID)
	}

	return mix, nil
}

// ExportComposition 导出作品
func (s *Service) ExportComposition(ctx context.Context, id string, format string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comp, ok := s.compositions[id]
	if !ok {
		return nil, fmt.Errorf("composition not found: %s", id)
	}

	// 导出为JSON格式
	return json.MarshalIndent(comp, "", "  ")
}

// AnalyzeComposition 分析作品
func (s *Service) AnalyzeComposition(ctx context.Context, id string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	comp, ok := s.compositions[id]
	if !ok {
		return nil, fmt.Errorf("composition not found: %s", id)
	}

	totalNotes := 0
	for _, track := range comp.Tracks {
		totalNotes += len(track.Notes)
	}

	analysis := map[string]interface{}{
		"composition_id": comp.ID,
		"title":         comp.Title,
		"genre":         comp.Genre,
		"mood":          comp.Mood,
		"bpm":           comp.BPM,
		"key":           comp.Key,
		"duration":      comp.Duration,
		"track_count":   len(comp.Tracks),
		"total_notes":   totalNotes,
		"complexity":    s.calculateComplexity(comp),
		"energy_level":  s.calculateEnergyLevel(comp),
		"harmonic_analysis": s.analyzeHarmony(comp),
	}

	return analysis, nil
}

// calculateComplexity 计算复杂度
func (s *Service) calculateComplexity(comp *Composition) float64 {
	if len(comp.Tracks) == 0 {
		return 0.0
	}

	totalNotes := 0
	for _, track := range comp.Tracks {
		totalNotes += len(track.Notes)
	}

	// 复杂度基于音符密度和音轨数
	avgNotesPerTrack := float64(totalNotes) / float64(len(comp.Tracks))
	complexity := math.Min(avgNotesPerTrack/100.0, 1.0)

	return complexity
}

// calculateEnergyLevel 计算能量级别
func (s *Service) calculateEnergyLevel(comp *Composition) float64 {
	if len(comp.Tracks) == 0 {
		return 0.0
	}

	totalVelocity := 0
	totalNotes := 0
	for _, track := range comp.Tracks {
		for _, note := range track.Notes {
			totalVelocity += note.Velocity
			totalNotes++
		}
	}

	if totalNotes == 0 {
		return 0.0
	}

	avgVelocity := float64(totalVelocity) / float64(totalNotes)
	return avgVelocity / 127.0
}

// analyzeHarmony 和声分析
func (s *Service) analyzeHarmony(comp *Composition) map[string]interface{} {
	return map[string]interface{}{
		"key":           comp.Key,
		"time_signature": comp.TimeSignature,
		"chord_progression": s.detectChordProgression(comp),
		"harmonic_tension": s.calculateHarmonicTension(comp),
	}
}

// detectChordProgression 检测和弦进行
func (s *Service) detectChordProgression(comp *Composition) []string {
	// 简化版本，返回常见和弦进行
	switch comp.Mood {
	case MoodHappy:
		return []string{"I", "V", "vi", "IV"}
	case MoodSad:
		return []string{"vi", "IV", "I", "V"}
	case MoodEnergetic:
		return []string{"I", "IV", "V", "I"}
	default:
		return []string{"I", "IV", "vi", "V"}
	}
}

// calculateHarmonicTension 计算和声张力
func (s *Service) calculateHarmonicTension(comp *Composition) float64 {
	// 基于BPM和调性计算张力
	bpmFactor := float64(comp.BPM) / 200.0
	return math.Min(bpmFactor, 1.0)
}
