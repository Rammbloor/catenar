package appshell

import "catenar/internal/contracts"

type BootstrapResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *BootstrapData           `json:"data,omitempty"`
	Error *contracts.ErrorEnvelope `json:"error,omitempty"`
}

type ProbeResponse struct {
	Ok    bool                     `json:"ok"`
	Data  *ProbeAcknowledgement    `json:"data,omitempty"`
	Error *contracts.ErrorEnvelope `json:"error,omitempty"`
}

type BootstrapData struct {
	App        AppMetadata                  `json:"app"`
	Contract   contracts.ContractManifest   `json:"contract"`
	Workspace  *contracts.WorkspaceSnapshot `json:"workspace,omitempty"`
	Layout     LayoutDefinition             `json:"layout"`
	StateModel StateModelSummary            `json:"stateModel"`
	EpicZero   []SliceStatus                `json:"epicZero"`
}

type AppMetadata struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	ProductLine  string `json:"productLine"`
	Platform     string `json:"platform"`
	Architecture string `json:"architecture"`
	GoVersion    string `json:"goVersion"`
	WailsVersion string `json:"wailsVersion"`
}

type LayoutDefinition struct {
	Regions []LayoutRegion `json:"regions"`
}

type LayoutRegion struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
}

type StateModelSummary struct {
	PrimaryFlow             []string `json:"primaryFlow"`
	OverlayViews            []string `json:"overlayViews"`
	SingleActiveLiveSession bool     `json:"singleActiveLiveSession"`
}

type SliceStatus struct {
	Slice   string `json:"slice"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type ProbeAcknowledgement struct {
	EventID        string `json:"eventId"`
	EventName      string `json:"eventName"`
	EmittedAt      string `json:"emittedAt"`
	Classification string `json:"classification"`
}
