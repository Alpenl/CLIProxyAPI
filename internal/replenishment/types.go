package replenishment

type CallbackConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

type CreateJobInput struct {
	RequestedSuccesses int
	Source             string
	ZipRequired        bool
	Callback           *CallbackConfig
}

type createJobRequest struct {
	RequestedSuccesses int             `json:"requestedSuccesses"`
	Source             string          `json:"source"`
	ZipRequired        bool            `json:"zipRequired"`
	Callback           *CallbackConfig `json:"callback,omitempty"`
}

type Job struct {
	ID                 string `json:"id"`
	Status             string `json:"status"`
	RequestedSuccesses int    `json:"requestedSuccesses"`
	SuccessCount       int    `json:"successCount"`
	FailureCount       int    `json:"failureCount"`
	ArchivePath        string `json:"archivePath"`
}

type PoolSnapshot struct {
	Target   int
	Healthy  int
	Warming  int
	Cooling  int
	Invalid  int
	Reserved int
	Total    int
}
