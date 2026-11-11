package rest

import "github.com/tienduc120/voice-recorder/services/recorder/internal/application"

type statusDTO struct {
	State         string    `json:"state"`
	RecordedMs    int64     `json:"recordedMs"`
	Level         float64   `json:"level"`
	Folder        string    `json:"folder"`
	FolderProblem string    `json:"folderProblem"`
	NoiseCleanup  bool      `json:"noiseCleanup"`
	Mics          []micDTO  `json:"mics"`
	MicID         string    `json:"micId"`
	File          fileDTO   `json:"file"`
	Notice        noticeDTO `json:"notice"`
	DefaultName   string    `json:"defaultName"`
}

type micDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`
}

type fileDTO struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type noticeDTO struct {
	Kind        string `json:"kind"`
	Mic         string `json:"mic"`
	PreviousMic string `json:"previousMic"`
}

type startRequest struct {
	Title string `json:"title"`
}

type settingsRequest struct {
	Folder       string `json:"folder"`
	NoiseCleanup bool   `json:"noiseCleanup"`
}

type micRequest struct {
	ID string `json:"id"`
}

type browseResponse struct {
	Folder string `json:"folder"`
	Chosen bool   `json:"chosen"`
}

type levelResponse struct {
	Level float64 `json:"level"`
}

type pingResponse struct {
	App string `json:"app"`
}

func statusOf(st application.Status) statusDTO {
	mics := make([]micDTO, 0, len(st.Mics))
	for _, m := range st.Mics {
		mics = append(mics, micDTO{ID: m.ID, Name: m.Name, IsDefault: m.IsDefault})
	}
	return statusDTO{
		State:         string(st.State),
		RecordedMs:    st.Recorded.Milliseconds(),
		Level:         st.Level,
		Folder:        st.Folder,
		FolderProblem: st.FolderProblem,
		NoiseCleanup:  st.NoiseCleanup,
		Mics:          mics,
		MicID:         st.MicID,
		File:          fileDTO{Name: st.File.Name, Size: st.File.Size},
		Notice:        noticeDTO{Kind: string(st.Notice.Kind), Mic: st.Notice.Mic, PreviousMic: st.Notice.PreviousMic},
		DefaultName:   st.DefaultName,
	}
}
