package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lngdao/social-dl/internal/deps"
	"github.com/lngdao/social-dl/internal/updater"
	"github.com/lngdao/social-dl/internal/ytdlp"
)

type viewState int

const (
	viewSetup viewState = iota
	viewUpdatePrompt
	viewHome
	viewSingle
	viewBatch
	viewProfile
	viewSettings
	viewInfo
	viewProgress
	viewHistory
)

// Update check messages
type updateCheckMsg struct{ version string }
type updateDoneMsg struct{ err error }
type updateSkipMsg struct{}

type App struct {
	state    viewState
	setup    setupModel
	home     homeModel
	single   singleModel
	batch    batchModel
	profile  profileModel
	settings settingsViewModel
	info     infoModel
	progress progressModel
	history  historyModel

	paths       deps.Paths
	needsSetup  bool
	version     string
	appSettings AppSettings

	width, height   int
	currentURL      string
	currentPlatform string
	program         *tea.Program
	updateVersion   string // available update version, empty if none
	updating        bool   // currently updating
}

func NewApp(version string) (*App, error) {
	paths, status, err := deps.Check()
	if err != nil {
		return nil, err
	}

	s := loadSettings()

	app := &App{
		paths:       paths,
		needsSetup:  status.NeedsSetup(),
		version:     version,
		appSettings: s,
	}

	// Always pass through the setup screen. On existing installs this is a
	// quick, throttled yt-dlp update check; on first run it downloads deps.
	app.state = viewSetup
	app.setup = newSetupModel(paths, status)

	return app, nil
}

func (a *App) SetProgram(p *tea.Program) {
	a.program = p
}

func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{a.checkForUpdate()}
	switch a.state {
	case viewSetup:
		cmds = append(cmds, a.setup.Init())
	}
	return tea.Batch(cmds...)
}

func (a App) checkForUpdate() tea.Cmd {
	ver := a.version
	return func() tea.Msg {
		latest, err := updater.CheckLatest()
		if err != nil || !updater.IsNewer(ver, latest) {
			return updateSkipMsg{}
		}
		return updateCheckMsg{version: latest}
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Global
	switch msg := msg.(type) {
	case updateCheckMsg:
		// Update available — show prompt (unless in setup)
		if a.state == viewSetup {
			a.updateVersion = msg.version // show after setup
			return &a, nil
		}
		a.updateVersion = msg.version
		a.state = viewUpdatePrompt
		return &a, nil

	case updateSkipMsg:
		return &a, nil

	case updateDoneMsg:
		a.updating = false
		if msg.err != nil {
			// Update failed, continue to home
			a.state = viewHome
			a.home = newHomeModel(a.version, a.appSettings)
			return &a, nil
		}
		// Updated successfully, quit so user restarts
		return &a, tea.Quit

	case tea.KeyMsg:
		// Handle update prompt
		if a.state == viewUpdatePrompt {
			switch msg.String() {
			case "y", "Y", "enter":
				a.updating = true
				return &a, func() tea.Msg {
					err := updater.SelfUpdate(nil)
					return updateDoneMsg{err: err}
				}
			case "n", "N", "esc", "q":
				return a.goHome()
			}
			return &a, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return &a, tea.Quit
		case "q":
			if a.state == viewHome {
				return &a, tea.Quit
			}
			if a.state == viewProgress && (a.progress.status == "finished" || a.progress.status == "error") {
				return &a, tea.Quit
			}
		case "esc":
			switch a.state {
			case viewSingle, viewBatch, viewProfile, viewSettings, viewHistory:
				return a.goHome()
			case viewInfo:
				// Back to single input
				a.state = viewSingle
				a.single = newSingleModel()
				return &a, a.single.Init()
			case viewProgress:
				if a.progress.status == "finished" || a.progress.status == "error" {
					return a.goHome()
				}
			}
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
	}

	switch a.state {
	case viewSetup:
		return a.updateSetup(msg)
	case viewHome:
		return a.updateHome(msg)
	case viewSingle:
		return a.updateSingle(msg)
	case viewBatch:
		return a.updateBatch(msg)
	case viewProfile:
		return a.updateProfile(msg)
	case viewSettings:
		return a.updateSettings(msg)
	case viewInfo:
		return a.updateInfo(msg)
	case viewProgress:
		return a.updateProgress(msg)
	case viewHistory:
		return a.updateHistory(msg)
	}

	return &a, nil
}

func (a App) goHome() (tea.Model, tea.Cmd) {
	a.appSettings = loadSettings()
	a.state = viewHome
	a.home = newHomeModel(a.version, a.appSettings)
	return &a, nil
}

// --- Setup ---

func (a App) updateSetup(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.setup, cmd = a.setup.Update(msg)
	if a.setup.done {
		paths, _, _ := deps.Check()
		a.paths = paths
		// If update was detected during setup, show prompt now
		if a.updateVersion != "" {
			a.state = viewUpdatePrompt
			return &a, nil
		}
		return a.goHome()
	}
	return &a, cmd
}

// --- Home ---

func (a App) updateHome(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case menuSelectMsg:
		switch msg.action {
		case menuSingle:
			a.state = viewSingle
			a.single = newSingleModel()
			return &a, a.single.Init()
		case menuBatch:
			a.state = viewBatch
			a.batch = newBatchModel()
			return &a, a.batch.Init()
		case menuProfile:
			a.state = viewProfile
			a.profile = newProfileModel()
			return &a, a.profile.Init()
		case menuSettings:
			a.state = viewSettings
			a.settings = newSettingsViewModel(a.appSettings)
			return &a, a.settings.Init()
		case menuHistory:
			a.state = viewHistory
			a.history = newHistoryModel()
			return &a, nil
		}
	}
	var cmd tea.Cmd
	a.home, cmd = a.home.Update(msg)
	return &a, cmd
}

// --- Single ---

func (a App) updateSingle(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case submitURLMsg:
		a.currentURL = msg.url
		a.state = viewInfo
		a.info = newInfoModel(msg.url, a.appSettings.IncludeAudio)
		return &a, tea.Batch(a.info.Init(), a.fetchMeta(msg.url))
	}
	var cmd tea.Cmd
	a.single, cmd = a.single.Update(msg)
	return &a, cmd
}

// --- Batch ---

func (a App) updateBatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case submitBatchMsg:
		if len(msg.urls) == 0 {
			a.batch.err = "Khong tim thay URL hop le"
			return &a, nil
		}
		a.state = viewProgress
		a.progress = newProgressModel(
			fmt.Sprintf("Tai %d video", len(msg.urls)), true)
		return &a, tea.Batch(a.progress.Init(), a.startBatchDownload(msg.urls, msg.subfolder))
	}
	var cmd tea.Cmd
	a.batch, cmd = a.batch.Update(msg)
	return &a, cmd
}

// --- Profile ---

func (a App) updateProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case profileScanMsg:
		if msg.entries != nil || msg.err != nil {
			a.profile, _ = a.profile.Update(msg)
			return &a, nil
		}
		// Start actual scan
		url := a.profile.input.Value()
		return &a, a.scanProfile(url)

	case submitProfileMsg:
		urls := make([]string, 0, len(msg.entries))
		for _, e := range msg.entries {
			if e.URL != "" {
				urls = append(urls, e.URL)
			}
		}
		if len(urls) == 0 {
			// Fallback: use profile URL directly for yt-dlp playlist
			urls = []string{msg.url}
		}
		subfolder := msg.subfolder
		a.state = viewProgress
		a.progress = newProgressModel(
			fmt.Sprintf("Tai %d video tu profile", len(urls)), true)
		return &a, tea.Batch(a.progress.Init(), a.startBatchDownload(urls, subfolder))
	}

	var cmd tea.Cmd
	a.profile, cmd = a.profile.Update(msg)
	return &a, cmd
}

// --- Settings ---

func (a App) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case settingsSavedMsg:
		a.appSettings = loadSettings()
	}
	var cmd tea.Cmd
	a.settings, cmd = a.settings.Update(msg)
	return &a, cmd
}

// --- Info ---

func (a App) updateInfo(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case metaFetchedMsg:
		if msg.meta != nil {
			a.currentPlatform = msg.meta.Extractor
		}
	case startDownloadMsg:
		a.state = viewProgress
		a.progress = newProgressModel(msg.meta.Title, false)
		a.currentPlatform = msg.meta.Extractor
		return &a, tea.Batch(a.progress.Init(), a.startSingleDownload(msg.meta, msg.quality, msg.includeAudio))
	}
	var cmd tea.Cmd
	a.info, cmd = a.info.Update(msg)
	return &a, cmd
}

// --- Progress ---

func (a App) updateProgress(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if a.progress.status == "finished" || a.progress.status == "error" {
				return a.goHome()
			}
		}
	case downloadDoneMsg:
		SaveHistory(HistoryEntry{
			Title:      a.progress.title,
			URL:        a.currentURL,
			FilePath:   msg.filePath,
			Platform:   a.currentPlatform,
			DownloadAt: time.Now(),
		})
	}
	var cmd tea.Cmd
	a.progress, cmd = a.progress.Update(msg)
	return &a, cmd
}

// --- History ---

func (a App) updateHistory(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	a.history, cmd = a.history.Update(msg)
	return &a, cmd
}

// --- View ---

func (a App) View() string {
	header := logoView() + "  " + mutedStyle.Render("v"+a.version) + "\n\n"

	var content string
	switch a.state {
	case viewSetup:
		return a.setup.View()
	case viewUpdatePrompt:
		return header + a.updatePromptView()
	case viewHome:
		return a.home.View()
	case viewSingle:
		content = a.single.View()
	case viewBatch:
		content = a.batch.View()
	case viewProfile:
		content = a.profile.View()
	case viewSettings:
		content = a.settings.View()
	case viewInfo:
		content = a.info.View()
	case viewProgress:
		content = a.progress.View()
	case viewHistory:
		content = a.history.View()
	}

	return header + content
}

func (a App) updatePromptView() string {
	if a.updating {
		return activeBoxStyle.Render(
			lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).
				Render("Dang cap nhat...") + "\n\n" +
				mutedStyle.Render("Tai phien ban moi tu GitHub..."),
		)
	}
	return activeBoxStyle.Render(
		warnStyle.Render("Co phien ban moi!") + "\n\n" +
			mutedStyle.Render("Hien tai: ") +
			lipgloss.NewStyle().Foreground(colorText).Render("v"+a.version) + "\n" +
			mutedStyle.Render("Moi nhat: ") +
			successStyle.Render(a.updateVersion) + "\n\n" +
			lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Cap nhat ngay? [Y/n]"),
	)
}

// ================== Commands ==================

func (a App) fetchMeta(url string) tea.Cmd {
	ytdlpPath := a.paths.YtDlp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		meta, err := ytdlp.FetchMeta(ctx, ytdlpPath, url)
		if err != nil {
			return metaErrorMsg{err: err}
		}
		return metaFetchedMsg{meta: meta}
	}
}

func (a App) formatSpec() string {
	s := a.appSettings
	switch s.Quality {
	case "1080p":
		return "bestvideo[height<=1080]+bestaudio/best[height<=1080]/best"
	case "720p":
		return "bestvideo[height<=720]+bestaudio/best[height<=720]/best"
	case "480p":
		return "bestvideo[height<=480]+bestaudio/best[height<=480]/best"
	case "360p":
		return "bestvideo[height<=360]+bestaudio/best[height<=360]/best"
	default:
		return "bestvideo*+bestaudio/best"
	}
}

func (a App) makeDownloadOpts(url string) ytdlp.DownloadOpts {
	opts := ytdlp.DownloadOpts{
		YtDlpPath:    a.paths.YtDlp,
		FfmpegDir:    a.paths.BinDir,
		URL:          url,
		FormatSpec:   a.formatSpec(),
		OutputDir:    a.appSettings.OutputDir,
		IncludeAudio: a.appSettings.IncludeAudio,
		CookieFile:   a.appSettings.CookieFile,
	}
	if a.appSettings.UseArchive {
		opts.ArchiveFile = archivePath()
	}
	if a.appSettings.VerboseLog {
		opts.LogFile = logFilePath()
	}
	return opts
}

func (a App) startSingleDownload(meta *ytdlp.VideoMeta, quality ytdlp.Quality, includeAudio bool) tea.Cmd {
	opts := a.makeDownloadOpts(a.currentURL)
	opts.FormatSpec = quality.FormatSpec
	opts.IncludeAudio = includeAudio
	program := a.program
	return func() tea.Msg {
		ctx := context.Background()
		filePath, err := ytdlp.Download(ctx, opts, func(p ytdlp.Progress) {
			if program != nil {
				program.Send(downloadProgressMsg{progress: p, index: -1})
			}
		})
		if err != nil {
			return downloadErrorMsg{err: err}
		}
		return downloadDoneMsg{filePath: filePath}
	}
}

type batchItem struct {
	index    int
	url      string
	title    string
	platform string
}

func (a App) startBatchDownload(urls []string, subfolder string) tea.Cmd {
	program := a.program
	settings := a.appSettings
	paths := a.paths
	fmtSpec := a.formatSpec()
	concurrency := settings.Concurrency
	if concurrency < 1 {
		concurrency = 3
	}

	return func() tea.Msg {
		outputDir := settings.OutputDir
		if subfolder != "" {
			outputDir = filepath.Join(outputDir, subfolder)
			os.MkdirAll(outputDir, 0755)
		}

		total := len(urls)

		// Phase 1: Fetch metadata (skip if setting enabled)
		items := make([]batchItem, total)
		for i, url := range urls {
			items[i] = batchItem{index: i, url: url, title: url}
		}

		if !settings.SkipMetadata {
			var metaWg sync.WaitGroup
			metaSem := make(chan struct{}, 5)
			for i, url := range urls {
				metaWg.Add(1)
				go func(idx int, u string) {
					defer metaWg.Done()
					metaSem <- struct{}{}
					defer func() { <-metaSem }()

					metaCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					meta, err := ytdlp.FetchMeta(metaCtx, paths.YtDlp, u)
					if err == nil && meta != nil {
						if meta.Title != "" {
							items[idx].title = meta.Title
						}
						items[idx].platform = meta.Extractor
					}
				}(i, url)
			}
			metaWg.Wait()
		}

		// Phase 2: Download all in parallel with concurrency limit
		var succeeded, failed atomic.Int32
		var dlWg sync.WaitGroup
		dlSem := make(chan struct{}, concurrency)

		for _, item := range items {
			dlWg.Add(1)
			go func(it batchItem) {
				defer dlWg.Done()
				dlSem <- struct{}{}
				defer func() { <-dlSem }()

				if program != nil {
					program.Send(batchItemStartMsg{
						index: it.index,
						total: total,
						title: it.title,
					})
				}

				opts := ytdlp.DownloadOpts{
					YtDlpPath:    paths.YtDlp,
					FfmpegDir:    paths.BinDir,
					URL:          it.url,
					FormatSpec:   fmtSpec,
					OutputDir:    outputDir,
					IncludeAudio: settings.IncludeAudio,
					CookieFile:   settings.CookieFile,
				}
				if settings.UseArchive {
					opts.ArchiveFile = archivePath()
				}
				if settings.VerboseLog {
					opts.LogFile = logFilePath()
				}

				// Download with 1 retry
				idx := it.index
				var filePath string
				var err error
				for attempt := 0; attempt < 2; attempt++ {
					ctx := context.Background()
					filePath, err = ytdlp.Download(ctx, opts, func(p ytdlp.Progress) {
						if program != nil {
							program.Send(downloadProgressMsg{progress: p, index: idx})
						}
					})
					if err == nil {
						break
					}
					if attempt == 0 {
						time.Sleep(2 * time.Second)
					}
				}

				if err != nil {
					failed.Add(1)
				} else {
					succeeded.Add(1)
					SaveHistory(HistoryEntry{
						Title:      it.title,
						URL:        it.url,
						FilePath:   filePath,
						Platform:   it.platform,
						DownloadAt: time.Now(),
					})
				}

				if program != nil {
					program.Send(batchItemDoneMsg{index: it.index, total: total})
				}
			}(item)
		}

		dlWg.Wait()
		return batchDoneMsg{
			succeeded: int(succeeded.Load()),
			failed:    int(failed.Load()),
		}
	}
}

func (a App) scanProfile(url string) tea.Cmd {
	ytdlpPath := a.paths.YtDlp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		entries, err := ytdlp.ScanPlaylist(ctx, ytdlpPath, url)
		return profileScanMsg{entries: entries, err: err}
	}
}
