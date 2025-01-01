package model

import (
	"fmt"
)

func (m *Model) handleEnter() {
	if m.State == MainMenu {
		m.lastMainCursor = m.Cursor
		switch m.Choices[m.Cursor] {
		case "list":
			m.SubChoices = []string{"option1", "option2"} // Reset to default options
			m.State = ListSubMenu
			m.Cursor = 0
		case "contexts":
			m.lastMainCursor = m.Cursor
			m.Message = m.handleContexts()
		case "pod":
			m.lastMainCursor = m.Cursor
			m.Message = m.handlePod()
		case "certificates":
			m.lastMainCursor = m.Cursor
			m.Message = m.handleCertificates()
		}
		return
	}

	if m.State == ListSubMenu {
		if m.Choices[m.lastMainCursor] == "contexts" {
			m.Message = m.handleContextSwitch()
			m.State = MainMenu
			return
		}

		if m.State == ListSubMenu && m.Choices[m.lastMainCursor] == "pod" {
			m.Message = m.handlePodSwitch()
			m.State = MainMenu
			return
		}

		//if m.Choices[m.lastMainCursor] == "certificates" {
		//	m.Message = m.handleCertificateDetails()
		//	m.State = MainMenu
		//	return
		//}
		if m.State == ListSubMenu {
			if m.Choices[m.lastMainCursor] == "certificates" {
				if m.Cursor == 0 { // Header row
					return
				}
				m.selectedCert = &m.certificates[m.Cursor-1]
				m.State = RenewalConfirm
				m.Message = fmt.Sprintf("Do you want to renew certificate %s/%s? (y/n)",
					m.selectedCert.Namespace,
					m.selectedCert.Name)
				return
			}
		}

		if m.State == RenewalConfirm {
			if m.renewalResponse == "y" {
				m.Message = m.handleCertificateRenewal()
			}
			m.State = MainMenu
			m.renewalResponse = "" // Reset the response
			return
		}

		switch m.SubChoices[m.Cursor] {
		case "option1":
			m.Message = m.handleOption1()
		case "option2":
			m.Message = m.handleOption2()
		}
	}
}

func (m *Model) handleOption1() string {
	return "Running list option1..."
}

func (m *Model) handleOption2() string {
	return "Running list option2..."
}

func (m *Model) handleContexts() string {
	contexts, err := m.contextCtl.GetContexts()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if len(contexts) == 0 {
		return "No contexts found"
	}
	m.SubChoices = contexts
	m.State = ListSubMenu
	return "Select context to switch to"
}

func (m *Model) handlePod() string {
	pods, err := m.contextCtl.GetPods()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if len(pods) == 0 {
		return "No pods found"
	}
	m.pods = pods
	m.SubChoices = make([]string, len(m.pods)+1) // +1 for the header

	// Create the header
	m.SubChoices[0] = "NAMESPACE\tNAME\tREADY\tSTATUS\tRESTARTS\tAGE"

	for i, pod := range m.pods {
		restarts := 0
		if len(pod.Pod.Status.ContainerStatuses) > 0 {
			restarts = int(pod.Pod.Status.ContainerStatuses[0].RestartCount)
		}

		restartStr := fmt.Sprintf("%d", restarts)
		if restarts > 0 {
			restartStr += fmt.Sprintf(" (%s ago)", pod.Age)
		}

		m.SubChoices[i+1] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
			pod.Pod.Namespace,
			pod.Pod.Name,
			pod.ReadyCount,
			pod.Pod.Status.Phase,
			restartStr,
			pod.Age)
	}

	m.State = ListSubMenu
	m.Cursor = 0
	return "Select pod to view"
}

func (m *Model) handlePodSwitch() string {
	if len(m.pods) == 0 {
		return "No pod available"
	}
	selectedPod := m.pods[m.Cursor]
	return fmt.Sprintf("Pod: %s\nNamespace: %s\nStatus: %s\nNode: %s\nReady: %s\nAge: %s",
		selectedPod.Pod.Name,
		selectedPod.Pod.Namespace,
		selectedPod.Pod.Status.Phase,
		selectedPod.Pod.Spec.NodeName,
		selectedPod.FormattedString,
		selectedPod.Age)
}

func (m *Model) handleContextSwitch() string {
	if len(m.SubChoices) == 0 {
		return "No context available"
	}
	selectedContext := m.SubChoices[m.Cursor]
	if err := m.contextCtl.SwitchContext(selectedContext); err != nil {
		return fmt.Sprintf("Error switching context: %v", err)
	}
	return fmt.Sprintf("Switched to context: %s", selectedContext)
}

func (m *Model) handleCertificates() string {
	certs, err := m.certCtl.GetTLSCertificates()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	if len(certs) == 0 {
		return "No certificates found"
	}

	m.certificates = certs
	m.SubChoices = make([]string, len(m.certificates)+1) // +1 for the header

	// Create the header
	m.SubChoices[0] = "NAMESPACE\tNAME\tEXPIRES IN\tSTATUS"

	for i, cert := range m.certificates {
		status := "Valid"
		if cert.IsExpired {
			status = "Expired"
		}

		m.SubChoices[i+1] = fmt.Sprintf("%s\t%s\t%d days\t%s",
			cert.Namespace,
			cert.Name,
			cert.DaysRemaining,
			status)
	}

	m.State = ListSubMenu
	m.Cursor = 0
	return "Select certificate to view details"
}

func (m *Model) handleCertificateDetails() string {
	if len(m.certificates) == 0 {
		return "No certificate available"
	}
	selectedCert := m.certificates[m.Cursor-1] // -1 because first row is header

	status := "Valid"
	if selectedCert.IsExpired {
		status = "Expired"
	}

	return fmt.Sprintf("Certificate Details:\nName: %s\nNamespace: %s\nValid From: %s\nValid Until: %s\nDays Remaining: %d\nStatus: %s\nSerial Number: %s",
		selectedCert.Name,
		selectedCert.Namespace,
		selectedCert.NotBefore.Format("2006-01-02 15:04:05"),
		selectedCert.NotAfter.Format("2006-01-02 15:04:05"),
		selectedCert.DaysRemaining,
		status,
		selectedCert.SerialNumber)
}

func (m *Model) handleCertificateRenewal() string {
	if m.selectedCert == nil {
		return "No certificate selected for renewal"
	}

	err := m.certCtl.RenewCertificate(m.selectedCert.Namespace, m.selectedCert.Name)
	if err != nil {
		return fmt.Sprintf("Failed to renew certificate: %v", err)
	}

	return fmt.Sprintf("Successfully renewed certificate %s/%s",
		m.selectedCert.Namespace,
		m.selectedCert.Name)
}
