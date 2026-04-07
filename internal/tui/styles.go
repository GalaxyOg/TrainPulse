package tui

import "github.com/charmbracelet/lipgloss"

type styles struct {
	root            lipgloss.Style
	header          lipgloss.Style
	headerMuted     lipgloss.Style
	panel           lipgloss.Style
	panelTitle      lipgloss.Style
	panelTitleDim   lipgloss.Style
	row             lipgloss.Style
	rowSelected     lipgloss.Style
	label           lipgloss.Style
	value           lipgloss.Style
	statusLine      lipgloss.Style
	helpBar         lipgloss.Style
	focusOn         lipgloss.Style
	focusOff        lipgloss.Style
	modal           lipgloss.Style
	modalTitle      lipgloss.Style
	modalHint       lipgloss.Style
	errText         lipgloss.Style
	okText          lipgloss.Style
	statusRunning   lipgloss.Style
	statusFailed    lipgloss.Style
	statusSuccess   lipgloss.Style
	statusInterrupt lipgloss.Style
	statusStopped   lipgloss.Style
	statusUnknown   lipgloss.Style
}

func defaultStyles() styles {
	border := lipgloss.RoundedBorder()
	return styles{
		root:        lipgloss.NewStyle().Padding(0, 1),
		header:      lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		headerMuted: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		panel: lipgloss.NewStyle().
			Border(border).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1),
		panelTitle:    lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
		panelTitleDim: lipgloss.NewStyle().Foreground(lipgloss.Color("243")),
		row:           lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		rowSelected: lipgloss.NewStyle().
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("25")).
			Bold(true),
		label:      lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		value:      lipgloss.NewStyle().Foreground(lipgloss.Color("252")),
		statusLine: lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236")).Padding(0, 1),
		helpBar:    lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("238")).Padding(0, 1),
		focusOn:    lipgloss.NewStyle().BorderForeground(lipgloss.Color("45")),
		focusOff:   lipgloss.NewStyle().BorderForeground(lipgloss.Color("238")),
		modal: lipgloss.NewStyle().
			Border(border).
			BorderForeground(lipgloss.Color("81")).
			Padding(1, 2).
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")),
		modalTitle:      lipgloss.NewStyle().Foreground(lipgloss.Color("117")).Bold(true),
		modalHint:       lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
		errText:         lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		okText:          lipgloss.NewStyle().Foreground(lipgloss.Color("84")).Bold(true),
		statusRunning:   lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),
		statusFailed:    lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true),
		statusSuccess:   lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true),
		statusInterrupt: lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true),
		statusStopped:   lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true),
		statusUnknown:   lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
	}
}
