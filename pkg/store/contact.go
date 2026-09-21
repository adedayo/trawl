package store

import "context"

// AssetExposureHistory is the durable, per-service result of Change 007's
// exposure accounting. Observed and inferred durations remain separate; a
// caller must not collapse them into one measured figure.
type AssetExposureHistory struct {
	AssetID                 string `json:"assetId"`
	Service                 string `json:"service"`
	FirstObserved           string `json:"firstObserved"`
	LastObserved            string `json:"lastObserved"`
	StillExposed            bool   `json:"stillExposed"`
	LeftCensored            bool   `json:"leftCensored"`
	ObservedDurationSeconds int64  `json:"observedDurationSeconds"`
	InferredDurationSeconds int64  `json:"inferredDurationSeconds"`
	BlindDurationSeconds    int64  `json:"blindDurationSeconds"`
	ExpectedBlindSeconds    int64  `json:"expectedBlindSeconds"`
	WorstBlindSeconds       int64  `json:"worstBlindSeconds"`
}

// ExposureHistoryStore persists measured exposure history, never attacker
// contact posterior telemetry.
type ExposureHistoryStore interface {
	SaveExposureHistory(context.Context, *AssetExposureHistory) error
	GetExposureHistory(context.Context, string) ([]AssetExposureHistory, error)
}
