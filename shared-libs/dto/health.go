package dto

// HealthResponse describes the payload returned by standard /healthz endpoints.
type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Version string `json:"version"`
}
