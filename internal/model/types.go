package model

import (
	tea "github.com/charmbracelet/bubbletea"
	"kubegreen/internal/controller"
)

type MenuState int

const (
	MainMenu MenuState = iota
	ListSubMenu
	RenewalConfirm
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

	return &Model{
		Choices:    []string{"list", "contexts", "pod", "certificates"},
		State:      MainMenu,
		contextCtl: ctlr,
		certCtl:    certCtl,
	}
}
