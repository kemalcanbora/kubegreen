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

	return &Model{
		Choices:    []string{"list", "contexts", "pod", "certificates", "volumes"},
		State:      MainMenu,
		contextCtl: ctlr,
		certCtl:    certCtl,
		volumeCtl:  volumeCtl,
	}
}
