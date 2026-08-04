package microinit

type ServiceDef struct {
	Name             string         `json:"name"`
	Enabled          *bool          `json:"enabled,omitempty"`
	Daemon           *bool          `json:"daemon,omitempty"`
	Restart          *bool          `json:"restart,omitempty"`
	RestartBackoff   *int           `json:"restartBackoff,omitempty"`
	StartWaitSecs    *int           `json:"startWaitSecs,omitempty"`
	ShutdownWaitSecs *int           `json:"shutdownWaitSecs,omitempty"`
	DependsOn        []string       `json:"dependsOn,omitempty"`
	StartCmd         string         `json:"startCmd,omitempty"`
	StopCmd          string         `json:"stopCmd,omitempty"`
	Cmd              string         `json:"cmd,omitempty"`
	Cwd              string         `json:"cwd,omitempty"`
	LivenessProbe    *LivenessProbe `json:"livenessProbe,omitempty"`
}

type LivenessProbe struct {
	HTTPUrl           string `json:"httpUrl,omitempty"`
	HTTPAcceptedCodes []int  `json:"httpAcceptedCodes,omitempty"`
	TCPAddr           string `json:"tcpAddr,omitempty"`
	Cmd               string `json:"cmd,omitempty"`
	SuccessExitCodes  []int  `json:"successExitCodes,omitempty"`
	Interval          int    `json:"interval,omitempty"`
	Timeout           int    `json:"timeout,omitempty"`
}

type DropinFile struct {
	Services []ServiceDef `json:"services"`
}
