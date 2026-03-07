package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/xuri/excelize/v2"
	"go.uber.org/zap"
)

const dayMillis int64 = 24 * 60 * 60 * 1000

type appState int

const (
	stateTargetInput appState = iota
	stateLoading
	stateIPSelect
	stateMenu
	stateGroupView
	stateGroupSelect
	stateNumberInput
	stateClientSelect
	stateQuickConfirm
	stateProcessing
	stateResult
)

type actionKind int

const (
	actionNone actionKind = iota
	actionGroupRenew
	actionExpiredWindowRenew
	actionUpcomingWindowRenew
	actionExportXLSX
)

type numberPurpose int

const (
	purposeNone numberPurpose = iota
	purposeGroupRenewDays
	purposeExpiredWindowDays
	purposeExpiredRenewDays
	purposeUpcomingWindowDays
	purposeUpcomingRenewDays
)

type endpointConfig struct {
	Origin    string
	BasePath  string
	APIBase   string
	Username  string
	Password  string
	TwoFA     string
	PanelHost string
}

type apiMsg struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

type inboundDTO struct {
	ID             int    `json:"id"`
	Remark         string `json:"remark"`
	Protocol       string `json:"protocol"`
	Settings       string `json:"settings"`
	StreamSettings string `json:"streamSettings"`
	Tag            string `json:"tag"`
	Port           int    `json:"port"`
	Listen         string `json:"listen"`
}

type outboundInfo struct {
	Tag      string
	Protocol string
	Address  string
	Port     int
}

type clientRecord struct {
	Key            string
	InboundID      int
	InboundRemark  string
	InboundTag     string
	Protocol       string
	Listen         string
	Port           int
	Email          string
	ClientID       string
	ClientAPIKey   string
	Security       string
	Flow           string
	Enable         bool
	ExpiryTime     int64
	CreatedAt      int64
	UpdatedAt      int64
	TotalGB        int64
	ClientObject   map[string]any
	StreamSettings map[string]any
	OutboundTag    string
	EgressIP       string
	VmessLink      string
	GroupKey       string
}

type panelData struct {
	Inbounds []inboundDTO
	Config   map[string]any
	Records  []*clientRecord
}

type threeXUIClient struct {
	ep      endpointConfig
	client  *http.Client
	logger  *zap.Logger
	timeout time.Duration
}

type ipStat struct {
	IP      string
	Total   int
	Expired int
	Soon    int
}

type groupStat struct {
	Key       string
	Total     int
	Expired   int
	Unexpired int
}

type nearestExpiryGroup struct {
	Key           string
	Total         int
	ExpiredToday  int
	NearestExpiry int64
}

type renewRequest struct {
	RecordKey   string
	InboundID   int
	Protocol    string
	ClientAPIID string
	ClientMap   map[string]any
	OldExpiry   int64
}

type exportRow struct {
	InboundRemark string
	Email         string
	VmessLink     string
	EgressIP      string
	OpenAt        string
	ExpiryAt      string
}

type loadDoneMsg struct {
	Err        error
	TargetLine string
	Endpoint   endpointConfig
	Client     *threeXUIClient
	Panel      *panelData
}

type renewDoneMsg struct {
	Updated int
	Failed  int
	Errors  []string
	Changed map[string]int64
}

type exportDoneMsg struct {
	Err   error
	Path  string
	Count int
}

type model struct {
	logger *zap.Logger

	targetLine string
	timeout    time.Duration

	state appState
	err   string

	targetInput   textinput.Model
	numberInput   textinput.Model
	numberHint    string
	numberPrompt  string
	numberPurpose numberPurpose

	spin spinner.Model

	endpoint endpointConfig
	api      *threeXUIClient
	panel    *panelData
	records  []*clientRecord

	ipStats     []ipStat
	ipCursor    int
	selectedIPs map[string]bool

	menuCursor int

	groups         []groupStat
	groupCursor    int
	selectedGroups map[string]bool

	candidates         []*clientRecord
	candidateCursor    int
	candidatePageSize  int
	selectedCandidates map[string]bool
	autoRenewRecords   []*clientRecord

	pendingAction actionKind
	windowDays    int
	renewDays     int

	resultTitle string
	resultLines []string

	quickRenewDays int
	quickTargets   []*clientRecord

	processingText string
}

var menuItems = []string{
	"Prefix grouping overview",
	"Batch renew by selected groups",
	"Batch renew expired within X days",
	"Batch renew expiring within X days",
	"Quick renew today expired (one confirm)",
	"Quick renew expiring in 3 days (paged)",
	"Export XLSX",
	"Re-select egress IP",
	"Reload panel data",
	"Quit",
}

func main() {
	var target string
	var timeoutSec int
	var logPath string
	var quickDays int
	var renewAllDays int
	var renewExpiredDays int
	var renewUpcomingDays int
	var exportVmessXLSX bool
	var exportPath string

	root := &cobra.Command{
		Use:   "threeuitui",
		Short: "3x-ui terminal renew/export tool",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := newLogger(logPath)
			if err != nil {
				return err
			}
			defer func() { _ = logger.Sync() }()

			if renewAllDays > 0 {
				if exportVmessXLSX {
					return errors.New("use only one of --renew-all-days, --renew-expired-days, --export-vmess-xlsx")
				}
				if renewExpiredDays > 0 {
					return errors.New("use only one of --renew-all-days or --renew-expired-days")
				}
				if renewUpcomingDays > 0 {
					return errors.New("use only one of --renew-all-days, --renew-expired-days or --renew-upcoming-days")
				}
				if strings.TrimSpace(target) == "" {
					return errors.New("--target is required when --renew-all-days is set")
				}
				summary, err := runDirectRenewAll(logger, strings.TrimSpace(target), time.Duration(timeoutSec)*time.Second, renewAllDays)
				fmt.Println(summary)
				return err
			}

			if renewExpiredDays > 0 {
				if exportVmessXLSX {
					return errors.New("use only one of --renew-all-days, --renew-expired-days, --export-vmess-xlsx")
				}
				if renewUpcomingDays > 0 {
					return errors.New("use only one of --renew-all-days, --renew-expired-days or --renew-upcoming-days")
				}
				if strings.TrimSpace(target) == "" {
					return errors.New("--target is required when --renew-expired-days is set")
				}
				summary, err := runDirectRenewExpired(logger, strings.TrimSpace(target), time.Duration(timeoutSec)*time.Second, renewExpiredDays)
				fmt.Println(summary)
				return err
			}

			if renewUpcomingDays > 0 {
				if exportVmessXLSX {
					return errors.New("use only one of --renew-upcoming-days or --export-vmess-xlsx")
				}
				if strings.TrimSpace(target) == "" {
					return errors.New("--target is required when --renew-upcoming-days is set")
				}
				summary, err := runDirectRenewUpcoming(logger, strings.TrimSpace(target), time.Duration(timeoutSec)*time.Second, renewUpcomingDays)
				fmt.Println(summary)
				return err
			}

			if exportVmessXLSX {
				if strings.TrimSpace(target) == "" {
					return errors.New("--target is required when --export-vmess-xlsx is set")
				}
				outPath := strings.TrimSpace(exportPath)
				if outPath == "" {
					outPath = filepath.Join(".", fmt.Sprintf("3xui-vmess-export-%s.xlsx", time.Now().Format("20060102-150405")))
				}
				summary, err := runDirectExportVmess(logger, strings.TrimSpace(target), time.Duration(timeoutSec)*time.Second, outPath)
				fmt.Println(summary)
				return err
			}

			m := newModel(logger, strings.TrimSpace(target), time.Duration(timeoutSec)*time.Second, quickDays)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return err
			}
			return nil
		},
	}

	root.Flags().StringVar(&target, "target", "", "Target line: <url-with-basepath> <username> <password> [2fa]")
	root.Flags().IntVar(&timeoutSec, "timeout", 20, "HTTP timeout seconds")
	root.Flags().StringVar(&logPath, "log", "./threeui-tui.log", "zap log file path")
	root.Flags().IntVar(&quickDays, "quick-days", 30, "Default renew days for quick today-expired renew")
	root.Flags().IntVar(&renewAllDays, "renew-all-days", 0, "Non-interactive: renew ALL clients with expiry by N days")
	root.Flags().IntVar(&renewExpiredDays, "renew-expired-days", 0, "Non-interactive: renew ONLY expired clients by N days")
	root.Flags().IntVar(&renewUpcomingDays, "renew-upcoming-days", 0, "Non-interactive: renew unexpired clients expiring within N days by N days")
	root.Flags().BoolVar(&exportVmessXLSX, "export-vmess-xlsx", false, "Non-interactive: export VMESS dedicated lines to XLSX (Chinese headers)")
	root.Flags().StringVar(&exportPath, "export-path", "", "Output path for --export-vmess-xlsx")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newLogger(path string) (*zap.Logger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{path}
	cfg.ErrorOutputPaths = []string{path}
	return cfg.Build()
}

func newModel(logger *zap.Logger, initialTarget string, timeout time.Duration, quickDays int) model {
	ti := textinput.New()
	ti.Placeholder = "http://host:port/basepath user pass [2fa]"
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 100
	ti.SetValue(initialTarget)

	ni := textinput.New()
	ni.CharLimit = 10
	ni.Width = 20

	sp := spinner.New()
	sp.Spinner = spinner.Line

	m := model{
		logger:             logger,
		targetLine:         initialTarget,
		timeout:            timeout,
		quickRenewDays:     quickDays,
		candidatePageSize:  50,
		state:              stateTargetInput,
		targetInput:        ti,
		numberInput:        ni,
		spin:               sp,
		selectedIPs:        map[string]bool{},
		selectedGroups:     map[string]bool{},
		selectedCandidates: map[string]bool{},
	}

	if initialTarget != "" {
		m.state = stateLoading
		m.processingText = "Logging in and loading panel data..."
	}

	return m
}

func (m model) Init() tea.Cmd {
	if m.state == stateLoading {
		return tea.Batch(m.spin.Tick, loadPanelCmd(m.targetLine, m.timeout, m.logger))
	}
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch typed := msg.(type) {
	case tea.KeyMsg:
		if typed.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case loadDoneMsg:
		if typed.Err != nil {
			m.err = typed.Err.Error()
			m.state = stateTargetInput
			m.processingText = ""
			m.targetInput.Focus()
			return m, nil
		}
		m.err = ""
		m.targetLine = typed.TargetLine
		m.endpoint = typed.Endpoint
		m.api = typed.Client
		m.panel = typed.Panel
		m.records = typed.Panel.Records
		m.ipStats = buildIPStats(m.records)
		m.selectedIPs = map[string]bool{}
		for _, one := range m.ipStats {
			m.selectedIPs[one.IP] = false
		}
		m.ipCursor = 0
		m.state = stateIPSelect
		m.processingText = ""
		return m, nil
	case renewDoneMsg:
		m.state = stateResult
		m.resultTitle = "Renew completed"
		m.resultLines = []string{
			fmt.Sprintf("Updated: %d", typed.Updated),
			fmt.Sprintf("Failed: %d", typed.Failed),
		}
		if len(typed.Errors) > 0 {
			maxErr := len(typed.Errors)
			if maxErr > 8 {
				maxErr = 8
			}
			m.resultLines = append(m.resultLines, "Errors:")
			m.resultLines = append(m.resultLines, typed.Errors[:maxErr]...)
			if len(typed.Errors) > maxErr {
				m.resultLines = append(m.resultLines, fmt.Sprintf("...and %d more", len(typed.Errors)-maxErr))
			}
		}
		for _, rec := range m.records {
			if newExp, ok := typed.Changed[rec.Key]; ok {
				rec.ExpiryTime = newExp
			}
		}
		m.ipStats = buildIPStats(m.records)
		m.processingText = ""
		return m, nil
	case exportDoneMsg:
		m.state = stateResult
		if typed.Err != nil {
			m.resultTitle = "Export failed"
			m.resultLines = []string{typed.Err.Error()}
		} else {
			m.resultTitle = "Export completed"
			m.resultLines = []string{
				fmt.Sprintf("Rows: %d", typed.Count),
				fmt.Sprintf("Path: %s", typed.Path),
			}
		}
		m.processingText = ""
		return m, nil
	}

	switch m.state {
	case stateTargetInput:
		return m.updateTargetInput(msg)
	case stateLoading, stateProcessing:
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd
	case stateIPSelect:
		return m.updateIPSelect(msg)
	case stateMenu:
		return m.updateMenu(msg)
	case stateGroupView:
		return m.updateGroupView(msg)
	case stateGroupSelect:
		return m.updateGroupSelect(msg)
	case stateNumberInput:
		return m.updateNumberInput(msg)
	case stateClientSelect:
		return m.updateClientSelect(msg)
	case stateQuickConfirm:
		return m.updateQuickConfirm(msg)
	case stateResult:
		return m.updateResult(msg)
	default:
		return m, nil
	}
}

func (m model) updateTargetInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.targetInput, cmd = m.targetInput.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			line := strings.TrimSpace(m.targetInput.Value())
			if line == "" {
				m.err = "Target line cannot be empty"
				return m, nil
			}
			m.err = ""
			m.targetLine = line
			m.state = stateLoading
			m.processingText = "Logging in and loading panel data..."
			return m, tea.Batch(m.spin.Tick, loadPanelCmd(line, m.timeout, m.logger))
		}
	}
	return m, cmd
}

func (m model) updateIPSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.ipCursor > 0 {
				m.ipCursor--
			}
		case "down", "j":
			if m.ipCursor < len(m.ipStats)-1 {
				m.ipCursor++
			}
		case " ":
			if len(m.ipStats) > 0 {
				ip := m.ipStats[m.ipCursor].IP
				m.selectedIPs[ip] = !m.selectedIPs[ip]
			}
		case "a":
			allSelected := true
			for _, row := range m.ipStats {
				if !m.selectedIPs[row.IP] {
					allSelected = false
					break
				}
			}
			for _, row := range m.ipStats {
				m.selectedIPs[row.IP] = !allSelected
			}
		case "enter":
			if len(m.getSelectedIPs()) == 0 {
				m.err = "Please select at least one egress IP"
				return m, nil
			}
			m.err = ""
			m.state = stateMenu
			m.menuCursor = 0
		case "r":
			m.state = stateLoading
			m.processingText = "Reloading panel data..."
			return m, tea.Batch(m.spin.Tick, loadPanelCmd(m.targetLine, m.timeout, m.logger))
		}
	}
	return m, nil
}

func (m model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(menuItems)-1 {
				m.menuCursor++
			}
		case "enter":
			return m.onMenuEnter()
		case "i":
			m.state = stateIPSelect
		case "y":
			return m.prepareTodayExpiredQuickConfirm()
		}
	}
	return m, nil
}

func (m model) onMenuEnter() (tea.Model, tea.Cmd) {
	switch m.menuCursor {
	case 0:
		m.groups = buildGroups(m.filteredRecords())
		m.groupCursor = 0
		m.state = stateGroupView
		return m, nil
	case 1:
		m.groups = buildGroups(m.filteredRecords())
		if len(m.groups) == 0 {
			m.state = stateResult
			m.resultTitle = "No data"
			m.resultLines = []string{"No record under selected egress IP"}
			return m, nil
		}
		m.groupCursor = 0
		m.selectedGroups = map[string]bool{}
		for _, g := range m.groups {
			m.selectedGroups[g.Key] = false
		}
		m.state = stateGroupSelect
		return m, nil
	case 2:
		m.pendingAction = actionExpiredWindowRenew
		m.prepareNumberInput(purposeExpiredWindowDays, "Enter X days for expired window", "7")
		return m, nil
	case 3:
		m.pendingAction = actionUpcomingWindowRenew
		m.prepareNumberInput(purposeUpcomingWindowDays, "Enter X days for upcoming window", "7")
		return m, nil
	case 4:
		return m.prepareTodayExpiredQuickConfirm()
	case 5:
		m.pendingAction = actionUpcomingWindowRenew
		m.windowDays = 3
		m.prepareNumberInput(purposeUpcomingRenewDays, "Enter renew days for expiring-in-3-days clients", "30")
		return m, nil
	case 6:
		return m.executeExport()
	case 7:
		m.state = stateIPSelect
		return m, nil
	case 8:
		m.state = stateLoading
		m.processingText = "Reloading panel data..."
		return m, tea.Batch(m.spin.Tick, loadPanelCmd(m.targetLine, m.timeout, m.logger))
	case 9:
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m model) prepareTodayExpiredQuickConfirm() (tea.Model, tea.Cmd) {
	targets := todayExpiredRecords(m.filteredRecords())
	if len(targets) == 0 {
		m.state = stateResult
		m.resultTitle = "No data"
		m.resultLines = []string{"No clients expired today under selected egress IP"}
		return m, nil
	}
	m.quickTargets = targets
	m.state = stateQuickConfirm
	return m, nil
}

func (m model) updateGroupView(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.groupCursor > 0 {
				m.groupCursor--
			}
		case "down", "j":
			if m.groupCursor < len(m.groups)-1 {
				m.groupCursor++
			}
		case "esc", "enter":
			m.state = stateMenu
		}
	}
	return m, nil
}

func (m model) updateGroupSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.groupCursor > 0 {
				m.groupCursor--
			}
		case "down", "j":
			if m.groupCursor < len(m.groups)-1 {
				m.groupCursor++
			}
		case " ":
			if len(m.groups) > 0 {
				keyName := m.groups[m.groupCursor].Key
				m.selectedGroups[keyName] = !m.selectedGroups[keyName]
			}
		case "a":
			allSelected := true
			for _, g := range m.groups {
				if !m.selectedGroups[g.Key] {
					allSelected = false
					break
				}
			}
			for _, g := range m.groups {
				m.selectedGroups[g.Key] = !allSelected
			}
		case "enter":
			selected := 0
			for _, g := range m.groups {
				if m.selectedGroups[g.Key] {
					selected++
				}
			}
			if selected == 0 {
				m.err = "Please select at least one group"
				return m, nil
			}
			m.err = ""
			m.pendingAction = actionGroupRenew
			m.prepareNumberInput(purposeGroupRenewDays, "Enter renew days", "30")
		case "esc":
			m.state = stateMenu
		}
	}
	return m, nil
}

func (m *model) prepareNumberInput(purpose numberPurpose, prompt, defaultValue string) {
	m.numberPurpose = purpose
	m.numberPrompt = prompt
	m.numberHint = "Press Enter to confirm"
	m.numberInput.SetValue(defaultValue)
	m.numberInput.CursorEnd()
	m.numberInput.Focus()
	m.state = stateNumberInput
}

func (m model) updateNumberInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.numberInput, cmd = m.numberInput.Update(msg)
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.state = stateMenu
			m.err = ""
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.numberInput.Value())
			if val == "" {
				val = "0"
			}
			n, err := strconv.Atoi(val)
			if err != nil || n < 0 {
				m.err = "Please enter a non-negative integer"
				return m, nil
			}
			m.err = ""
			switch m.numberPurpose {
			case purposeGroupRenewDays:
				if n <= 0 {
					m.err = "Renew days must be greater than 0"
					return m, nil
				}
				m.renewDays = n
				return m.prepareGroupRenewCandidates()
			case purposeExpiredWindowDays:
				m.windowDays = n
				m.prepareNumberInput(purposeExpiredRenewDays, "Enter renew days", "30")
				return m, nil
			case purposeExpiredRenewDays:
				if n <= 0 {
					m.err = "Renew days must be greater than 0"
					return m, nil
				}
				m.renewDays = n
				return m.executeExpiredWindowRenew()
			case purposeUpcomingWindowDays:
				m.windowDays = n
				m.prepareNumberInput(purposeUpcomingRenewDays, "Enter renew days", "30")
				return m, nil
			case purposeUpcomingRenewDays:
				if n <= 0 {
					m.err = "Renew days must be greater than 0"
					return m, nil
				}
				m.renewDays = n
				return m.prepareUpcomingCandidates()
			}
		}
	}
	return m, cmd
}

func (m model) prepareGroupRenewCandidates() (tea.Model, tea.Cmd) {
	filtered := m.filteredRecords()
	selectedGroupMap := map[string]bool{}
	for k, v := range m.selectedGroups {
		if v {
			selectedGroupMap[k] = true
		}
	}
	now := time.Now().UnixMilli()
	expired := make([]*clientRecord, 0)
	unexpired := make([]*clientRecord, 0)
	for _, rec := range filtered {
		if !selectedGroupMap[rec.GroupKey] {
			continue
		}
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= now {
			expired = append(expired, rec)
		} else {
			unexpired = append(unexpired, rec)
		}
	}

	if len(expired) == 0 && len(unexpired) == 0 {
		m.state = stateResult
		m.resultTitle = "No data"
		m.resultLines = []string{"No client matched selected groups"}
		return m, nil
	}

	if len(unexpired) == 0 {
		m.state = stateProcessing
		m.processingText = "Renewing expired clients..."
		return m, tea.Batch(m.spin.Tick, renewCmd(m.api, buildRenewRequests(expired), m.renewDays, m.timeout, m.logger))
	}

	m.candidates = unexpired
	m.autoRenewRecords = expired
	m.candidateCursor = 0
	m.selectedCandidates = map[string]bool{}
	for _, rec := range unexpired {
		m.selectedCandidates[rec.Key] = false
	}
	m.state = stateClientSelect
	return m, nil
}

func (m model) executeExpiredWindowRenew() (tea.Model, tea.Cmd) {
	filtered := m.filteredRecords()
	now := time.Now().UnixMilli()
	windowStart := now - int64(m.windowDays)*dayMillis
	targets := make([]*clientRecord, 0)
	for _, rec := range filtered {
		if rec.ExpiryTime <= 0 {
			continue
		}
		if rec.ExpiryTime <= now && rec.ExpiryTime >= windowStart {
			targets = append(targets, rec)
		}
	}
	if len(targets) == 0 {
		m.state = stateResult
		m.resultTitle = "No data"
		m.resultLines = []string{"No expired client matched this window"}
		return m, nil
	}
	m.state = stateProcessing
	m.processingText = "Renewing expired window clients..."
	return m, tea.Batch(m.spin.Tick, renewCmd(m.api, buildRenewRequests(targets), m.renewDays, m.timeout, m.logger))
}

func (m model) prepareUpcomingCandidates() (tea.Model, tea.Cmd) {
	filtered := m.filteredRecords()
	now := time.Now().UnixMilli()
	windowEnd := now + int64(m.windowDays)*dayMillis
	targets := make([]*clientRecord, 0)
	for _, rec := range filtered {
		if rec.ExpiryTime <= now {
			continue
		}
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= windowEnd {
			targets = append(targets, rec)
		}
	}
	if len(targets) == 0 {
		m.state = stateResult
		m.resultTitle = "No data"
		m.resultLines = []string{"No upcoming-expiry client matched this window"}
		return m, nil
	}
	m.candidates = targets
	m.autoRenewRecords = nil
	m.candidateCursor = 0
	m.selectedCandidates = map[string]bool{}
	for _, rec := range targets {
		m.selectedCandidates[rec.Key] = false
	}
	m.state = stateClientSelect
	return m, nil
}

func (m model) executeExport() (tea.Model, tea.Cmd) {
	rows := buildVmessExportRows(m.filteredRecords())
	if len(rows) == 0 {
		m.state = stateResult
		m.resultTitle = "No data"
		m.resultLines = []string{"No VMESS row to export under selected egress IP"}
		return m, nil
	}
	outPath := filepath.Join(".", fmt.Sprintf("3xui-vmess-export-%s.xlsx", time.Now().Format("20060102-150405")))
	m.state = stateProcessing
	m.processingText = "Exporting XLSX..."
	return m, tea.Batch(m.spin.Tick, exportCmd(outPath, rows))
}

func (m model) updateClientSelect(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.candidateCursor > 0 {
				m.candidateCursor--
			}
		case "down", "j":
			if m.candidateCursor < len(m.candidates)-1 {
				m.candidateCursor++
			}
		case "left", "pgup", "p":
			m.jumpCandidatePage(-1)
		case "right", "pgdown", "n":
			m.jumpCandidatePage(1)
		case " ":
			if len(m.candidates) > 0 {
				keyName := m.candidates[m.candidateCursor].Key
				m.selectedCandidates[keyName] = !m.selectedCandidates[keyName]
			}
		case "v":
			m.toggleCurrentCandidatePage()
		case "a":
			allSelected := true
			for _, rec := range m.candidates {
				if !m.selectedCandidates[rec.Key] {
					allSelected = false
					break
				}
			}
			for _, rec := range m.candidates {
				m.selectedCandidates[rec.Key] = !allSelected
			}
		case "enter":
			selected := make([]*clientRecord, 0)
			for _, rec := range m.candidates {
				if m.selectedCandidates[rec.Key] {
					selected = append(selected, rec)
				}
			}
			allTargets := append([]*clientRecord{}, m.autoRenewRecords...)
			allTargets = append(allTargets, selected...)
			if len(allTargets) == 0 {
				m.err = "No client selected"
				return m, nil
			}
			m.err = ""
			m.state = stateProcessing
			m.processingText = "Renewing selected clients..."
			return m, tea.Batch(m.spin.Tick, renewCmd(m.api, buildRenewRequests(allTargets), m.renewDays, m.timeout, m.logger))
		case "esc":
			m.state = stateMenu
		}
	}
	return m, nil
}

func (m model) updateQuickConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.state = stateMenu
			m.quickTargets = nil
			return m, nil
		case "enter", "y":
			if len(m.quickTargets) == 0 {
				m.state = stateResult
				m.resultTitle = "No data"
				m.resultLines = []string{"No clients expired today under selected egress IP"}
				return m, nil
			}
			m.state = stateProcessing
			m.processingText = fmt.Sprintf("Quick renewing today-expired clients (%d)...", len(m.quickTargets))
			requests := buildRenewRequests(m.quickTargets)
			m.quickTargets = nil
			return m, tea.Batch(m.spin.Tick, renewCmd(m.api, requests, m.quickRenewDays, m.timeout, m.logger))
		}
	}
	return m, nil
}

func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter", "esc":
			m.state = stateMenu
			m.resultTitle = ""
			m.resultLines = nil
		}
	}
	return m, nil
}

func (m model) View() string {
	var b strings.Builder
	b.WriteString("3x-ui Terminal Tool (BubbleTea + Zap + CLI)\n")
	b.WriteString(strings.Repeat("=", 56))
	b.WriteString("\n")
	if m.err != "" {
		b.WriteString("Error: ")
		b.WriteString(m.err)
		b.WriteString("\n\n")
	}

	switch m.state {
	case stateTargetInput:
		b.WriteString("Input target line:\n")
		b.WriteString("format: <url-with-basepath> <username> <password> [2fa]\n\n")
		b.WriteString(m.targetInput.View())
		b.WriteString("\n\nEnter: load panel, Ctrl+C: quit\n")
	case stateLoading, stateProcessing:
		b.WriteString(fmt.Sprintf("%s %s\n", m.spin.View(), m.processingText))
		b.WriteString("\nPlease wait...\n")
	case stateIPSelect:
		b.WriteString("Step 1: select egress IP(s) for this run\n")
		b.WriteString("Up/Down: move, Space: toggle, A: toggle all, Enter: confirm, R: reload\n\n")
		for i, row := range m.ipStats {
			cursor := " "
			if i == m.ipCursor {
				cursor = ">"
			}
			box := "[ ]"
			if m.selectedIPs[row.IP] {
				box = "[x]"
			}
			b.WriteString(fmt.Sprintf("%s %s %-24s total=%d expired=%d soon7d=%d\n", cursor, box, row.IP, row.Total, row.Expired, row.Soon))
		}
		if len(m.ipStats) == 0 {
			b.WriteString("No clients loaded\n")
		} else {
			b.WriteString("\nNearest-expiry groups (all records):\n")
			for _, g := range buildNearestExpiryGroups(m.records, 8) {
				b.WriteString(fmt.Sprintf("- %-28s nearest=%s total=%d expiredToday=%d\n", g.Key, formatMillis(g.NearestExpiry), g.Total, g.ExpiredToday))
			}
		}
	case stateMenu:
		filtered := m.filteredRecords()
		todayExpired := todayExpiredRecords(filtered)
		b.WriteString(fmt.Sprintf("Selected egress IPs: %s\n", strings.Join(m.getSelectedIPs(), ", ")))
		b.WriteString(fmt.Sprintf("Today expired (selected IP scope): %d | Quick renew days: %d\n", len(todayExpired), m.quickRenewDays))
		b.WriteString("\nNearest-expiry groups (selected IP scope):\n")
		for _, g := range buildNearestExpiryGroups(filtered, 8) {
			b.WriteString(fmt.Sprintf("- %-28s nearest=%s total=%d expiredToday=%d\n", g.Key, formatMillis(g.NearestExpiry), g.Total, g.ExpiredToday))
		}
		b.WriteString("\nMain menu (Up/Down + Enter, I to reselect IP, Y quick renew today-expired):\n")
		for i, it := range menuItems {
			cursor := " "
			if i == m.menuCursor {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %s\n", cursor, it))
		}
	case stateGroupView:
		b.WriteString("Prefix grouping by inbound_prefix + email_prefix\n")
		b.WriteString("Esc/Enter to return\n\n")
		for i, row := range m.groups {
			cursor := " "
			if i == m.groupCursor {
				cursor = ">"
			}
			b.WriteString(fmt.Sprintf("%s %-30s total=%d expired=%d unexpired=%d\n", cursor, row.Key, row.Total, row.Expired, row.Unexpired))
		}
	case stateGroupSelect:
		b.WriteString("Select groups for batch renew\n")
		b.WriteString("Up/Down: move, Space: toggle, A: toggle all, Enter: confirm\n\n")
		for i, row := range m.groups {
			cursor := " "
			if i == m.groupCursor {
				cursor = ">"
			}
			box := "[ ]"
			if m.selectedGroups[row.Key] {
				box = "[x]"
			}
			b.WriteString(fmt.Sprintf("%s %s %-30s total=%d expired=%d\n", cursor, box, row.Key, row.Total, row.Expired))
		}
	case stateNumberInput:
		b.WriteString(m.numberPrompt)
		b.WriteString("\n")
		b.WriteString(m.numberInput.View())
		b.WriteString("\n")
		b.WriteString(m.numberHint)
	case stateClientSelect:
		b.WriteString("Select clients to renew (unexpired list)\n")
		b.WriteString("Expired clients are auto-renewed.\n")
		b.WriteString("Up/Down: move, Space: toggle, V: toggle page, A: toggle all, PgUp/PgDn or P/N: page, Enter: run renew\n\n")
		if len(m.autoRenewRecords) > 0 {
			b.WriteString(fmt.Sprintf("Auto-renew expired count: %d\n\n", len(m.autoRenewRecords)))
		}
		start, end, page, totalPages := m.candidatePageWindow()
		b.WriteString(fmt.Sprintf("Candidates: %d | Selected: %d | Page: %d/%d\n\n", len(m.candidates), m.selectedCandidateCount(), page, totalPages))
		for i := start; i < end; i++ {
			rec := m.candidates[i]
			cursor := " "
			if i == m.candidateCursor {
				cursor = ">"
			}
			box := "[ ]"
			if m.selectedCandidates[rec.Key] {
				box = "[x]"
			}
			b.WriteString(fmt.Sprintf("%s %s #%d %-26s %-20s exp=%s ip=%s\n", cursor, box, i+1, rec.GroupKey, rec.Email, formatMillis(rec.ExpiryTime), rec.EgressIP))
		}
	case stateQuickConfirm:
		grouped := buildNearestExpiryGroups(m.quickTargets, 10)
		b.WriteString("Quick renew for TODAY expired clients\n")
		b.WriteString(fmt.Sprintf("Target clients: %d | Renew days: %d\n", len(m.quickTargets), m.quickRenewDays))
		b.WriteString("Press Enter (or Y) to confirm, Esc to cancel\n\n")
		for _, g := range grouped {
			b.WriteString(fmt.Sprintf("- %-28s count=%d expiredToday=%d\n", g.Key, g.Total, g.ExpiredToday))
		}
	case stateResult:
		b.WriteString(m.resultTitle)
		b.WriteString("\n\n")
		for _, line := range m.resultLines {
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\nPress Enter to return main menu\n")
	}

	return b.String()
}

func (m model) getSelectedIPs() []string {
	out := make([]string, 0)
	for _, row := range m.ipStats {
		if m.selectedIPs[row.IP] {
			out = append(out, row.IP)
		}
	}
	return out
}

func (m model) filteredRecords() []*clientRecord {
	selected := map[string]bool{}
	for _, ip := range m.getSelectedIPs() {
		selected[ip] = true
	}
	out := make([]*clientRecord, 0)
	for _, rec := range m.records {
		if selected[rec.EgressIP] {
			out = append(out, rec)
		}
	}
	return out
}

func (m model) selectedCandidateCount() int {
	count := 0
	for _, rec := range m.candidates {
		if m.selectedCandidates[rec.Key] {
			count++
		}
	}
	return count
}

func (m model) candidatePageWindow() (start, end, page, totalPages int) {
	total := len(m.candidates)
	if total == 0 {
		return 0, 0, 1, 1
	}
	size := m.candidatePageSize
	if size <= 0 {
		size = 50
	}
	cursor := m.candidateCursor
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	totalPages = (total + size - 1) / size
	page = cursor/size + 1
	start = (page - 1) * size
	end = start + size
	if end > total {
		end = total
	}
	return start, end, page, totalPages
}

func (m *model) jumpCandidatePage(delta int) {
	if len(m.candidates) == 0 {
		return
	}
	_, _, current, total := m.candidatePageWindow()
	target := current + delta
	if target < 1 {
		target = 1
	}
	if target > total {
		target = total
	}
	size := m.candidatePageSize
	if size <= 0 {
		size = 50
	}
	m.candidateCursor = (target - 1) * size
	if m.candidateCursor >= len(m.candidates) {
		m.candidateCursor = len(m.candidates) - 1
	}
}

func (m *model) toggleCurrentCandidatePage() {
	start, end, _, _ := m.candidatePageWindow()
	if end <= start {
		return
	}
	allSelected := true
	for i := start; i < end; i++ {
		if !m.selectedCandidates[m.candidates[i].Key] {
			allSelected = false
			break
		}
	}
	for i := start; i < end; i++ {
		m.selectedCandidates[m.candidates[i].Key] = !allSelected
	}
}

func loadPanelCmd(targetLine string, timeout time.Duration, logger *zap.Logger) tea.Cmd {
	return func() tea.Msg {
		ep, err := parseTargetLine(targetLine)
		if err != nil {
			return loadDoneMsg{Err: err}
		}
		client, err := newThreeXUIClient(ep, timeout, logger)
		if err != nil {
			return loadDoneMsg{Err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		if err := client.Login(ctx); err != nil {
			return loadDoneMsg{Err: err}
		}
		inbounds, err := client.GetInbounds(ctx)
		if err != nil {
			return loadDoneMsg{Err: err}
		}
		cfg, err := client.GetConfig(ctx)
		if err != nil {
			logger.Warn("getConfig failed, continue with empty config", zap.Error(err))
			cfg = map[string]any{}
		}
		records := buildRecords(inbounds, cfg, ep.PanelHost)
		return loadDoneMsg{
			TargetLine: targetLine,
			Endpoint:   client.ep,
			Client:     client,
			Panel: &panelData{
				Inbounds: inbounds,
				Config:   cfg,
				Records:  records,
			},
		}
	}
}

func renewCmd(client *threeXUIClient, requests []renewRequest, renewDays int, timeout time.Duration, logger *zap.Logger) tea.Cmd {
	return func() tea.Msg {
		return executeRenew(client, requests, renewDays, timeout, logger, nil)
	}
}

func executeRenew(client *threeXUIClient, requests []renewRequest, renewDays int, timeout time.Duration, logger *zap.Logger, progressFn func(done, total int, req renewRequest, err error)) renewDoneMsg {
	if len(requests) == 0 {
		return renewDoneMsg{}
	}
	opTimeout := timeout
	if opTimeout < 2*time.Minute {
		opTimeout = 2 * time.Minute
	}
	need := time.Duration(len(requests)) * 3 * time.Second
	if need > opTimeout {
		opTimeout = need
	}
	ctx, cancel := context.WithTimeout(context.Background(), opTimeout)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return renewDoneMsg{Failed: len(requests), Errors: []string{err.Error()}, Changed: map[string]int64{}}
	}

	updated := 0
	failed := 0
	errLines := make([]string, 0)
	changed := map[string]int64{}
	now := time.Now().UnixMilli()

	total := len(requests)
	for i, req := range requests {
		base := req.OldExpiry
		if base < now {
			base = now
		}
		newExpiry := base + int64(renewDays)*dayMillis
		err := client.UpdateClientExpiry(ctx, req.InboundID, req.ClientAPIID, req.ClientMap, newExpiry)
		if err != nil {
			failed++
			errLines = append(errLines, fmt.Sprintf("%s: %v", req.RecordKey, err))
			logger.Warn("renew failed", zap.String("record", req.RecordKey), zap.Error(err))
			if progressFn != nil {
				progressFn(i+1, total, req, err)
			}
			continue
		}
		updated++
		changed[req.RecordKey] = newExpiry
		if progressFn != nil {
			progressFn(i+1, total, req, nil)
		}
	}

	return renewDoneMsg{Updated: updated, Failed: failed, Errors: errLines, Changed: changed}
}

func runDirectRenewAll(logger *zap.Logger, targetLine string, timeout time.Duration, renewDays int) (string, error) {
	ep, err := parseTargetLine(targetLine)
	if err != nil {
		return "", err
	}
	client, err := newThreeXUIClient(ep, timeout, logger)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		return "", err
	}
	inbounds, err := client.GetInbounds(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := client.GetConfig(ctx)
	if err != nil {
		logger.Warn("getConfig failed, continue with empty config", zap.Error(err))
		cfg = map[string]any{}
	}
	records := buildRecords(inbounds, cfg, client.ep.PanelHost)
	targets := make([]*clientRecord, 0)
	for _, rec := range records {
		if rec.ExpiryTime > 0 {
			targets = append(targets, rec)
		}
	}
	if len(targets) == 0 {
		return fmt.Sprintf("Loaded clients=%d, renewable(expiry>0)=0", len(records)), nil
	}
	fmt.Printf("Start renew: loaded=%d renewable=%d days=%d\n", len(records), len(targets), renewDays)
	result := executeRenew(client, buildRenewRequests(targets), renewDays, timeout, logger, func(done, total int, req renewRequest, err error) {
		if err != nil {
			fmt.Printf("[%d/%d] FAIL %s -> %v\n", done, total, req.RecordKey, err)
			return
		}
		fmt.Printf("[%d/%d] OK %s\n", done, total, req.RecordKey)
	})
	summary := fmt.Sprintf("Loaded=%d Renewable=%d RenewDays=%d Updated=%d Failed=%d", len(records), len(targets), renewDays, result.Updated, result.Failed)
	if result.Failed > 0 {
		maxErr := len(result.Errors)
		if maxErr > 10 {
			maxErr = 10
		}
		summary = summary + "\nErrors:\n- " + strings.Join(result.Errors[:maxErr], "\n- ")
		if len(result.Errors) > maxErr {
			summary = summary + fmt.Sprintf("\n...and %d more", len(result.Errors)-maxErr)
		}
		return summary, fmt.Errorf("renew finished with %d failures", result.Failed)
	}
	return summary, nil
}

func runDirectRenewExpired(logger *zap.Logger, targetLine string, timeout time.Duration, renewDays int) (string, error) {
	ep, err := parseTargetLine(targetLine)
	if err != nil {
		return "", err
	}
	client, err := newThreeXUIClient(ep, timeout, logger)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		return "", err
	}
	inbounds, err := client.GetInbounds(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := client.GetConfig(ctx)
	if err != nil {
		logger.Warn("getConfig failed, continue with empty config", zap.Error(err))
		cfg = map[string]any{}
	}
	records := buildRecords(inbounds, cfg, client.ep.PanelHost)
	now := time.Now().UnixMilli()
	targets := make([]*clientRecord, 0)
	for _, rec := range records {
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= now {
			targets = append(targets, rec)
		}
	}
	if len(targets) == 0 {
		return fmt.Sprintf("Loaded clients=%d, expired=0", len(records)), nil
	}
	fmt.Printf("Start renew expired-only: loaded=%d expired=%d days=%d\n", len(records), len(targets), renewDays)
	result := executeRenew(client, buildRenewRequests(targets), renewDays, timeout, logger, func(done, total int, req renewRequest, err error) {
		if err != nil {
			fmt.Printf("[%d/%d] FAIL %s -> %v\n", done, total, req.RecordKey, err)
			return
		}
		fmt.Printf("[%d/%d] OK %s\n", done, total, req.RecordKey)
	})
	summary := fmt.Sprintf("Loaded=%d Expired=%d RenewDays=%d Updated=%d Failed=%d", len(records), len(targets), renewDays, result.Updated, result.Failed)
	if result.Failed > 0 {
		maxErr := len(result.Errors)
		if maxErr > 10 {
			maxErr = 10
		}
		summary = summary + "\nErrors:\n- " + strings.Join(result.Errors[:maxErr], "\n- ")
		if len(result.Errors) > maxErr {
			summary = summary + fmt.Sprintf("\n...and %d more", len(result.Errors)-maxErr)
		}
		return summary, fmt.Errorf("renew finished with %d failures", result.Failed)
	}
	return summary, nil
}

func runDirectRenewUpcoming(logger *zap.Logger, targetLine string, timeout time.Duration, renewDays int) (string, error) {
	ep, err := parseTargetLine(targetLine)
	if err != nil {
		return "", err
	}
	client, err := newThreeXUIClient(ep, timeout, logger)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		return "", err
	}
	inbounds, err := client.GetInbounds(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := client.GetConfig(ctx)
	if err != nil {
		logger.Warn("getConfig failed, continue with empty config", zap.Error(err))
		cfg = map[string]any{}
	}
	records := buildRecords(inbounds, cfg, client.ep.PanelHost)
	now := time.Now().UnixMilli()
	windowEnd := now + int64(renewDays)*dayMillis
	targets := make([]*clientRecord, 0)
	for _, rec := range records {
		if rec.ExpiryTime <= now {
			continue
		}
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= windowEnd {
			targets = append(targets, rec)
		}
	}
	if len(targets) == 0 {
		return fmt.Sprintf("Loaded clients=%d, upcomingWithin%dd=0", len(records), renewDays), nil
	}
	fmt.Printf("Start renew upcoming-only: loaded=%d upcomingWithin%dd=%d renewDays=%d\n", len(records), renewDays, len(targets), renewDays)
	result := executeRenew(client, buildRenewRequests(targets), renewDays, timeout, logger, func(done, total int, req renewRequest, err error) {
		if err != nil {
			fmt.Printf("[%d/%d] FAIL %s -> %v\n", done, total, req.RecordKey, err)
			return
		}
		fmt.Printf("[%d/%d] OK %s\n", done, total, req.RecordKey)
	})
	summary := fmt.Sprintf("Loaded=%d UpcomingWithin%dd=%d RenewDays=%d Updated=%d Failed=%d", len(records), renewDays, len(targets), renewDays, result.Updated, result.Failed)
	if result.Failed > 0 {
		maxErr := len(result.Errors)
		if maxErr > 10 {
			maxErr = 10
		}
		summary = summary + "\nErrors:\n- " + strings.Join(result.Errors[:maxErr], "\n- ")
		if len(result.Errors) > maxErr {
			summary = summary + fmt.Sprintf("\n...and %d more", len(result.Errors)-maxErr)
		}
		return summary, fmt.Errorf("renew finished with %d failures", result.Failed)
	}
	return summary, nil
}

func runDirectExportVmess(logger *zap.Logger, targetLine string, timeout time.Duration, outPath string) (string, error) {
	ep, err := parseTargetLine(targetLine)
	if err != nil {
		return "", err
	}
	client, err := newThreeXUIClient(ep, timeout, logger)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := client.Login(ctx); err != nil {
		return "", err
	}
	inbounds, err := client.GetInbounds(ctx)
	if err != nil {
		return "", err
	}
	cfg, err := client.GetConfig(ctx)
	if err != nil {
		logger.Warn("getConfig failed, continue with empty config", zap.Error(err))
		cfg = map[string]any{}
	}
	records := buildRecords(inbounds, cfg, client.ep.PanelHost)
	rows := buildVmessExportRows(records)
	if len(rows) == 0 {
		return fmt.Sprintf("Loaded=%d VMESS=0", len(records)), nil
	}
	if err := exportXLSX(outPath, rows); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(outPath)
	if err != nil {
		abs = outPath
	}
	return fmt.Sprintf("Loaded=%d VMESS=%d Path=%s", len(records), len(rows), abs), nil
}

func exportCmd(path string, rows []exportRow) tea.Cmd {
	return func() tea.Msg {
		if err := exportXLSX(path, rows); err != nil {
			return exportDoneMsg{Err: err}
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			abs = path
		}
		return exportDoneMsg{Path: abs, Count: len(rows)}
	}
}

func parseTargetLine(line string) (endpointConfig, error) {
	parts := strings.Fields(strings.TrimSpace(line))
	if len(parts) < 3 {
		return endpointConfig{}, errors.New("target must be: <url-with-basepath> <username> <password> [2fa]")
	}
	rawURL := strings.TrimSpace(parts[0])
	username := strings.TrimSpace(parts[1])
	password := strings.TrimSpace(parts[2])
	twoFA := ""
	if len(parts) >= 4 {
		twoFA = strings.TrimSpace(parts[3])
		twoFA = strings.TrimPrefix(twoFA, "2fa=")
		twoFA = strings.TrimPrefix(twoFA, "2FA=")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return endpointConfig{}, fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return endpointConfig{}, errors.New("invalid url: missing scheme or host")
	}
	basePath := u.Path
	if basePath == "" {
		basePath = "/"
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	if !strings.HasSuffix(basePath, "/") {
		basePath += "/"
	}
	origin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	apiBase := joinURL(origin, basePath+"panel/api")

	return endpointConfig{
		Origin:    origin,
		BasePath:  basePath,
		APIBase:   apiBase,
		Username:  username,
		Password:  password,
		TwoFA:     twoFA,
		PanelHost: u.Hostname(),
	}, nil
}

func newThreeXUIClient(ep endpointConfig, timeout time.Duration, logger *zap.Logger) (*threeXUIClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &threeXUIClient{
		ep: ep,
		client: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
		logger:  logger,
		timeout: timeout,
	}, nil
}

func (c *threeXUIClient) Login(ctx context.Context) error {
	msg, err := c.loginOnce(ctx)
	if err != nil && strings.HasPrefix(c.ep.Origin, "https://") && strings.Contains(strings.ToLower(err.Error()), "http response to https client") {
		c.logger.Warn("https endpoint appears plain-http, retrying with http", zap.String("origin", c.ep.Origin))
		c.setOrigin("http://" + strings.TrimPrefix(c.ep.Origin, "https://"))
		msg, err = c.loginOnce(ctx)
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "http response to https client") {
			return fmt.Errorf("%w (hint: this panel is http, use http:// not https://)", err)
		}
		return err
	}
	if !msg.Success {
		if msg.Msg == "" {
			msg.Msg = "login failed"
		}
		low := strings.ToLower(msg.Msg)
		if strings.Contains(low, "two-factor") && strings.TrimSpace(c.ep.TwoFA) == "" {
			return fmt.Errorf("%s (hint: panel may require 2FA, append code as 4th field: <url> <user> <pass> <2fa>)", msg.Msg)
		}
		return errors.New(msg.Msg)
	}
	return nil
}

func (c *threeXUIClient) loginOnce(ctx context.Context) (apiMsg, error) {
	loginURL := joinURL(c.ep.Origin, c.ep.BasePath+"login")
	form := url.Values{}
	form.Set("username", c.ep.Username)
	form.Set("password", c.ep.Password)
	if strings.TrimSpace(c.ep.TwoFA) != "" {
		form.Set("twoFactorCode", strings.TrimSpace(c.ep.TwoFA))
	}
	return c.postForm(ctx, loginURL, form)
}

func (c *threeXUIClient) setOrigin(origin string) {
	c.ep.Origin = origin
	c.ep.APIBase = joinURL(origin, c.ep.BasePath+"panel/api")
}

func (c *threeXUIClient) GetInbounds(ctx context.Context) ([]inboundDTO, error) {
	full := joinURL(c.ep.APIBase, "/inbounds/list")
	msg, err := c.getMsg(ctx, full)
	if err != nil {
		return nil, err
	}
	if !msg.Success {
		return nil, errors.New(msg.Msg)
	}
	var rows []inboundDTO
	if err := json.Unmarshal(msg.Obj, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *threeXUIClient) GetConfig(ctx context.Context) (map[string]any, error) {
	full := joinURL(c.ep.APIBase, "/server/getConfigJson")
	msg, err := c.getMsg(ctx, full)
	if err != nil {
		return nil, err
	}
	if !msg.Success {
		return nil, errors.New(msg.Msg)
	}
	var cfg map[string]any
	if len(msg.Obj) == 0 || string(msg.Obj) == "null" {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(msg.Obj, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *threeXUIClient) UpdateClientExpiry(ctx context.Context, inboundID int, clientAPIID string, clientMap map[string]any, newExpiry int64) error {
	if clientAPIID == "" {
		return errors.New("empty client api id")
	}
	clone := deepCopyMap(clientMap)
	clone["expiryTime"] = newExpiry
	clone["updated_at"] = time.Now().UnixMilli()

	settingsObj := map[string]any{
		"clients": []any{clone},
	}
	settingsBody, err := json.Marshal(settingsObj)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Set("id", strconv.Itoa(inboundID))
	form.Set("settings", string(settingsBody))

	urlPath := joinURL(c.ep.APIBase, "/inbounds/updateClient/"+url.PathEscape(clientAPIID))
	msg, err := c.postForm(ctx, urlPath, form)
	if err != nil {
		return err
	}
	if !msg.Success {
		if msg.Msg == "" {
			msg.Msg = "updateClient failed"
		}
		return errors.New(msg.Msg)
	}
	return nil
}

func (c *threeXUIClient) getMsg(ctx context.Context, endpoint string) (apiMsg, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return apiMsg{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return apiMsg{}, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiMsg{}, err
	}
	if resp.StatusCode >= 400 {
		return apiMsg{}, fmt.Errorf("http %d: %s", resp.StatusCode, string(bs))
	}
	var msg apiMsg
	if err := json.Unmarshal(bs, &msg); err != nil {
		return apiMsg{}, err
	}
	return msg, nil
}

func (c *threeXUIClient) postForm(ctx context.Context, endpoint string, form url.Values) (apiMsg, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return apiMsg{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	resp, err := c.client.Do(req)
	if err != nil {
		return apiMsg{}, err
	}
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiMsg{}, err
	}
	if resp.StatusCode >= 400 {
		return apiMsg{}, fmt.Errorf("http %d: %s", resp.StatusCode, string(bs))
	}
	var msg apiMsg
	if err := json.Unmarshal(bs, &msg); err != nil {
		return apiMsg{}, err
	}
	return msg, nil
}

func buildRecords(inbounds []inboundDTO, cfg map[string]any, panelHost string) []*clientRecord {
	rules := parseRoutingRules(cfg)
	defaultOutbound := parseDefaultOutbound(cfg)
	outboundMap := parseOutbounds(cfg)
	records := make([]*clientRecord, 0)

	for _, in := range inbounds {
		clients := parseClients(in.Settings)
		stream := parseJSONMap(in.StreamSettings)
		for _, clientObj := range clients {
			email := toString(clientObj["email"])
			id := toString(clientObj["id"])
			password := toString(clientObj["password"])
			protocol := strings.ToLower(strings.TrimSpace(in.Protocol))
			apiKey := clientAPIKey(protocol, id, password, email)
			if apiKey == "" {
				continue
			}
			outTag := resolveOutboundTag(rules, defaultOutbound, in.Tag, email)
			egIP := resolveEgressIP(outTag, outboundMap)
			rec := &clientRecord{
				Key:            fmt.Sprintf("%d|%s|%s", in.ID, email, apiKey),
				InboundID:      in.ID,
				InboundRemark:  in.Remark,
				InboundTag:     in.Tag,
				Protocol:       protocol,
				Listen:         in.Listen,
				Port:           in.Port,
				Email:          email,
				ClientID:       id,
				ClientAPIKey:   apiKey,
				Security:       toString(clientObj["security"]),
				Flow:           toString(clientObj["flow"]),
				Enable:         toBool(clientObj["enable"]),
				ExpiryTime:     toInt64(clientObj["expiryTime"]),
				CreatedAt:      toInt64(clientObj["created_at"]),
				UpdatedAt:      toInt64(clientObj["updated_at"]),
				TotalGB:        toInt64(clientObj["totalGB"]),
				ClientObject:   deepCopyMap(clientObj),
				StreamSettings: stream,
				OutboundTag:    outTag,
				EgressIP:       egIP,
				GroupKey:       prefixToken(in.Remark) + "|" + prefixToken(email),
			}
			rec.VmessLink = buildVmessLink(rec, panelHost)
			records = append(records, rec)
		}
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].EgressIP == records[j].EgressIP {
			if records[i].GroupKey == records[j].GroupKey {
				return records[i].Email < records[j].Email
			}
			return records[i].GroupKey < records[j].GroupKey
		}
		return records[i].EgressIP < records[j].EgressIP
	})

	return records
}

func parseClients(settings string) []map[string]any {
	root := parseJSONMap(settings)
	raw, ok := root["clients"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, one := range raw {
		if m, ok := one.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func parseJSONMap(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	out := map[string]any{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func parseRoutingRules(cfg map[string]any) []map[string]any {
	routing, _ := cfg["routing"].(map[string]any)
	rawRules, _ := routing["rules"].([]any)
	out := make([]map[string]any, 0, len(rawRules))
	for _, one := range rawRules {
		if m, ok := one.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func parseDefaultOutbound(cfg map[string]any) string {
	routing, _ := cfg["routing"].(map[string]any)
	if v := toString(routing["defaultOutboundTag"]); v != "" {
		return v
	}
	return ""
}

func parseOutbounds(cfg map[string]any) map[string]outboundInfo {
	out := map[string]outboundInfo{}
	raw, _ := cfg["outbounds"].([]any)
	for _, item := range raw {
		ob, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tag := toString(ob["tag"])
		if tag == "" {
			continue
		}
		protocol := toString(ob["protocol"])
		info := outboundInfo{Tag: tag, Protocol: protocol}
		if strings.EqualFold(protocol, "socks") {
			settings, _ := ob["settings"].(map[string]any)
			servers, _ := settings["servers"].([]any)
			if len(servers) > 0 {
				if sv, ok := servers[0].(map[string]any); ok {
					info.Address = toString(sv["address"])
					info.Port = int(toInt64(sv["port"]))
				}
			}
		}
		out[tag] = info
	}
	return out
}

func resolveOutboundTag(rules []map[string]any, fallback, inboundTag, email string) string {
	for _, rule := range rules {
		tag := toString(rule["outboundTag"])
		if tag == "" {
			continue
		}
		if hasUnsupportedRuleConditions(rule) {
			continue
		}
		if !stringListRuleMatch(rule["inboundTag"], inboundTag) {
			continue
		}
		if !stringListRuleMatch(rule["user"], email) {
			continue
		}
		return tag
	}
	return fallback
}

func hasUnsupportedRuleConditions(rule map[string]any) bool {
	checkKeys := []string{"domain", "ip", "port", "source", "sourcePort", "network", "protocol", "attrs"}
	for _, key := range checkKeys {
		if v, ok := rule[key]; ok {
			s := toString(v)
			if s != "" {
				return true
			}
			if arr, ok2 := v.([]any); ok2 && len(arr) > 0 {
				return true
			}
		}
	}
	return false
}

func stringListRuleMatch(ruleValue any, val string) bool {
	if ruleValue == nil {
		return true
	}
	arr, ok := ruleValue.([]any)
	if !ok {
		return true
	}
	if len(arr) == 0 {
		return true
	}
	val = strings.TrimSpace(strings.ToLower(val))
	for _, one := range arr {
		if strings.TrimSpace(strings.ToLower(toString(one))) == val {
			return true
		}
	}
	return false
}

func resolveEgressIP(outboundTag string, outbounds map[string]outboundInfo) string {
	if outboundTag == "" {
		return "unknown"
	}
	if info, ok := outbounds[outboundTag]; ok {
		if info.Address != "" {
			return info.Address
		}
	}
	return "unknown"
}

func clientAPIKey(protocol, id, password, email string) string {
	switch strings.ToLower(protocol) {
	case "trojan":
		return password
	case "shadowsocks":
		return email
	default:
		return id
	}
}

func buildVmessLink(rec *clientRecord, panelHost string) string {
	if rec.Protocol != "vmess" {
		return ""
	}
	if rec.ClientID == "" || rec.Email == "" {
		return ""
	}
	address := rec.Listen
	if address == "" || address == "0.0.0.0" || address == "::" || address == "::0" {
		address = panelHost
	}
	if address == "" {
		return ""
	}
	obj := map[string]any{
		"v":    "2",
		"ps":   fmt.Sprintf("%s-%s", rec.InboundRemark, rec.Email),
		"add":  address,
		"port": strconv.Itoa(rec.Port),
		"id":   rec.ClientID,
		"aid":  "0",
		"net":  "tcp",
		"type": "none",
		"host": "",
		"path": "",
		"tls":  "",
		"scy":  rec.Security,
	}
	stream := rec.StreamSettings
	if network := toString(stream["network"]); network != "" {
		obj["net"] = network
		switch network {
		case "ws":
			if ws, ok := stream["wsSettings"].(map[string]any); ok {
				obj["path"] = toString(ws["path"])
				if h := toString(ws["host"]); h != "" {
					obj["host"] = h
				} else if headers, ok2 := ws["headers"].(map[string]any); ok2 {
					obj["host"] = toString(headers["Host"])
				}
			}
		case "grpc":
			if grpc, ok := stream["grpcSettings"].(map[string]any); ok {
				obj["path"] = toString(grpc["serviceName"])
			}
		}
	}
	if sec := toString(stream["security"]); sec == "tls" {
		obj["tls"] = "tls"
		if tls, ok := stream["tlsSettings"].(map[string]any); ok {
			if sni := toString(tls["serverName"]); sni != "" {
				obj["sni"] = sni
			}
		}
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return ""
	}
	return "vmess://" + base64.StdEncoding.EncodeToString(body)
}

func buildIPStats(records []*clientRecord) []ipStat {
	now := time.Now().UnixMilli()
	soonLine := now + 7*dayMillis
	agg := map[string]*ipStat{}
	for _, rec := range records {
		ip := rec.EgressIP
		if ip == "" {
			ip = "unknown"
		}
		row, ok := agg[ip]
		if !ok {
			row = &ipStat{IP: ip}
			agg[ip] = row
		}
		row.Total++
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= now {
			row.Expired++
		}
		if rec.ExpiryTime > now && rec.ExpiryTime <= soonLine {
			row.Soon++
		}
	}
	out := make([]ipStat, 0, len(agg))
	for _, row := range agg {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].IP < out[j].IP
	})
	return out
}

func buildGroups(records []*clientRecord) []groupStat {
	now := time.Now().UnixMilli()
	agg := map[string]*groupStat{}
	for _, rec := range records {
		g := rec.GroupKey
		row, ok := agg[g]
		if !ok {
			row = &groupStat{Key: g}
			agg[g] = row
		}
		row.Total++
		if rec.ExpiryTime > 0 && rec.ExpiryTime <= now {
			row.Expired++
		} else {
			row.Unexpired++
		}
	}
	out := make([]groupStat, 0, len(agg))
	for _, row := range agg {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out
}

func todayExpiredRecords(records []*clientRecord) []*clientRecord {
	now := time.Now()
	start := dayStartMillis(now)
	nowMillis := now.UnixMilli()
	out := make([]*clientRecord, 0)
	for _, rec := range records {
		if rec.ExpiryTime <= 0 {
			continue
		}
		if rec.ExpiryTime >= start && rec.ExpiryTime <= nowMillis {
			out = append(out, rec)
		}
	}
	return out
}

func buildNearestExpiryGroups(records []*clientRecord, limit int) []nearestExpiryGroup {
	now := time.Now()
	start := dayStartMillis(now)
	nowMillis := now.UnixMilli()
	agg := map[string]*nearestExpiryGroup{}
	for _, rec := range records {
		key := rec.GroupKey
		if strings.TrimSpace(key) == "" {
			key = "unknown"
		}
		row, ok := agg[key]
		if !ok {
			row = &nearestExpiryGroup{Key: key}
			agg[key] = row
		}
		row.Total++
		if rec.ExpiryTime > 0 {
			if row.NearestExpiry == 0 || rec.ExpiryTime < row.NearestExpiry {
				row.NearestExpiry = rec.ExpiryTime
			}
			if rec.ExpiryTime >= start && rec.ExpiryTime <= nowMillis {
				row.ExpiredToday++
			}
		}
	}
	out := make([]nearestExpiryGroup, 0, len(agg))
	for _, row := range agg {
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		left := out[i].NearestExpiry
		right := out[j].NearestExpiry
		if left <= 0 && right > 0 {
			return false
		}
		if right <= 0 && left > 0 {
			return true
		}
		if left == right {
			return out[i].Key < out[j].Key
		}
		return left < right
	})
	if limit > 0 && len(out) > limit {
		return out[:limit]
	}
	return out
}

func dayStartMillis(now time.Time) int64 {
	year, month, day := now.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	return start.UnixMilli()
}

func buildRenewRequests(records []*clientRecord) []renewRequest {
	out := make([]renewRequest, 0, len(records))
	for _, rec := range records {
		out = append(out, renewRequest{
			RecordKey:   rec.Key,
			InboundID:   rec.InboundID,
			Protocol:    rec.Protocol,
			ClientAPIID: rec.ClientAPIKey,
			ClientMap:   deepCopyMap(rec.ClientObject),
			OldExpiry:   rec.ExpiryTime,
		})
	}
	return out
}

func buildVmessExportRows(records []*clientRecord) []exportRow {
	rows := make([]exportRow, 0)
	for _, rec := range records {
		if !strings.EqualFold(rec.Protocol, "vmess") {
			continue
		}
		if strings.TrimSpace(rec.VmessLink) == "" {
			continue
		}
		rows = append(rows, exportRow{
			InboundRemark: rec.InboundRemark,
			Email:         rec.Email,
			VmessLink:     rec.VmessLink,
			EgressIP:      rec.EgressIP,
			OpenAt:        formatMillis(rec.CreatedAt),
			ExpiryAt:      formatMillis(rec.ExpiryTime),
		})
	}
	return rows
}

func exportXLSX(path string, rows []exportRow) error {
	f := excelize.NewFile()
	sheet := "vmess"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"\u4e13\u7ebf\u5907\u6ce8",
		"\u8d26\u53f7",
		"VMESS\u4e13\u7ebf",
		"\u51fa\u53e3IP",
		"\u5f00\u901a\u65f6\u95f4",
		"\u5230\u671f\u65f6\u95f4",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
	}

	for i, row := range rows {
		r := i + 2
		vals := []any{row.InboundRemark, row.Email, row.VmessLink, row.EgressIP, row.OpenAt, row.ExpiryAt}
		for c, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(c+1, r)
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return err
			}
		}
	}

	for i := 1; i <= len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i)
		width := 22.0
		if i == 3 {
			width = 70
		}
		if err := f.SetColWidth(sheet, col, col, width); err != nil {
			return err
		}
	}

	return f.SaveAs(path)
}

func prefixToken(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "unknown"
	}
	tokens := strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case '-', '_', '.', '@', ':', '/', '\\':
			return true
		default:
			return false
		}
	})
	if len(tokens) == 0 {
		return s
	}
	return strings.TrimSpace(tokens[0])
}

func formatMillis(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return time.UnixMilli(ms).Format("2006-01-02 15:04:05")
}

func joinURL(origin, path string) string {
	return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(path, "/")
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(t, 10)
	case int:
		return strconv.Itoa(t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case json.Number:
		n, _ := t.Int64()
		return n
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		return n
	default:
		return 0
	}
}

func toBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		val := strings.TrimSpace(strings.ToLower(t))
		return val == "1" || val == "true"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	default:
		return false
	}
}

func deepCopyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	bs, err := json.Marshal(in)
	if err != nil {
		out := map[string]any{}
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	out := map[string]any{}
	if err := json.Unmarshal(bs, &out); err != nil {
		fallback := map[string]any{}
		for k, v := range in {
			fallback[k] = v
		}
		return fallback
	}
	return out
}
