package model

import (
	"kubegreen/internal/controller"

	tea "github.com/charmbracelet/bubbletea"
)

type MenuState int

const (
	MainMenu MenuState = iota
	ListSubMenu
	RenewalConfirm
	VolumeResizeMenu
	VolumeSizeInput
	MetricsView
)

type Model struct {
	Choices          []string
	SubChoices       []string
	Cursor           int
	State            MenuState
	Message          string
	contextCtl       *controller.ContextController
	lastMainCursor   int
	showDetails      bool
	pods             []controller.PodInfo
	selectedPodIndex int
	renewalResponse  string

	certCtl *controller.CertController

	selectedCert *controller.CertInfo
	certificates []controller.CertInfo

	// Volume-related fields
	volumeCtl      *controller.VolumeController
	volumes        []controller.VolumeInfo
	selectedVolume *controller.VolumeInfo
	newVolumeSize  string

	// Metrics-related fields
	metricsCtl *controller.MetricsController
	metrics    *controller.MetricsOutput
}

func NewModel() tea.Model {
	ctlr, err := controller.NewContextController()
	if err != nil {
		return &Model{
			Choices: []string{"Error: Cannot connect to kubernetes"},
			Message: err.Error(),
		}
	}

	certCtl := controller.NewCertController(ctlr.GetClientset())
	volumeCtl := controller.NewVolumeController(ctlr.GetClientset(), ctlr.GetConfig())
	metricsCtl := controller.NewMetricsController(ctlr.GetClientset(), ctlr.GetMetricsClientset())

	return &Model{
		Choices:    []string{"list", "contexts", "pod", "certificates", "volumes", "metrics"},
		State:      MainMenu,
		contextCtl: ctlr,
		certCtl:    certCtl,
		volumeCtl:  volumeCtl,
		metricsCtl: metricsCtl,
	}
}
