package shared

type Job struct {
	ID     string `json:"id"`
	Engine string `json:"engine"` // latex or typst
	Source string `json:"source"` // source file text
	Format string `json:"format"` // "pdf" or "png"
}

// type JobRequest struct {
// 	Engine string `json:"engine"` // latex or typst
// 	Source string `json:"source"` // source file text
// 	Format string `json:"format"` // "pdf" or "png"
// }

type JobInfo struct {
	Status    JobStatus `json:"status"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
	Engine    string    `json:"engine"`
	Format    string    `json:"format"`
	Error     string    `json:"error,omitempty"`
	ResultURL string    `json:"result_url,omitempty"`
}

type JobUpdate struct {
	UpdatedAt string    `json:"updated_at"`
	Status    JobStatus `json:"status"`
	Error     string    `json:"error,omitempty"`
	ResultURL string    `json:"result_url,omitempty"`
}

type JobPubSubMsg struct {
	ID        string    `json:"id"`
	Status    JobStatus `json:"status"`
	Error     string    `json:"error,omitempty"`
	ResultURL string    `json:"result_url,omitempty"`
}

type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusUploading  JobStatus = "uploading"
	JobStatusProcessing JobStatus = "processing"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCompleted  JobStatus = "completed"
)

// =================

func (j *JobPubSubMsg) ToMap() map[string]string {
	return map[string]string{
		"id":         j.ID,
		"status":     string(j.Status),
		"error":      j.Error,
		"result_url": j.ResultURL,
	}
}

func (j *JobUpdate) ToMap() map[string]string {
	return map[string]string{
		"updated_at": j.UpdatedAt,
		"status":     string(j.Status),
		"error":      j.Error,
		"result_url": j.ResultURL,
	}
}

func (j *JobInfo) ToMap() map[string]string {
	return map[string]string{
		"updated_at": j.UpdatedAt,
		"created_at": j.CreatedAt,
		"engine":     j.Engine,
		"format":     j.Format,
		"status":     string(j.Status),
		"error":      j.Error,
		"result_url": j.ResultURL,
	}
}
