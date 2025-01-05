package main

import (
	"fmt"
	"os"

	"kubegreen/internal/model"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	model := model.NewModel()
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
