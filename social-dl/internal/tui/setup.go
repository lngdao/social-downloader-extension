package tui

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/lngdao/social-dl/internal/deps"
)

type setupDoneMsg struct{ paths deps.Paths }
type setupErrMsg struct{ err error }

type setupModel struct {
	spinner     spinner.Model
	currentFile string
	fileIndex   int
	totalFiles  int
	done        bool
	err         error
	paths       deps.Paths
	status      deps.Status
	updateOnly  bool
}

func newSetupModel(paths deps.Paths, status deps.Status) setupModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPrimary)

	total := 0
	if status.NeedYtDlp {
		total++
	}
	if status.NeedFfmpeg {
		total++
	}
	if status.NeedFfprobe {
		total++
	}

	return setupModel{
		spinner:    s,
		paths:      paths,
		status:     status,
		totalFiles: total,
		updateOnly: !status.NeedsSetup(),
	}
}

func (m setupModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startDownload())
}

func (m setupModel) startDownload() tea.Cmd {
	paths := m.paths
	status := m.status
	return func() tea.Msg {
		if m.updateOnly {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, _ = deps.MaybeUpdateYtDlp(ctx, paths.YtDlp)
			return setupDoneMsg{paths: paths}
		}

		items, err := deps.PendingDownloads(paths, status)
		if err != nil {
			return setupErrMsg{err: err}
		}
		for _, item := range items {
			if err := deps.DownloadFile(item.URL, item.Dest, nil); err != nil {
				return setupErrMsg{err: fmt.Errorf("download %s: %w", item.Name, err)}
			}
			os.Chmod(item.Dest, 0755)
		}

		// If yt-dlp was already installed, refresh it after completing any
		// missing dependency setup. A newly downloaded yt-dlp is already the
		// latest release and does not need a second network request.
		if !status.NeedYtDlp {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			_, _ = deps.MaybeUpdateYtDlp(ctx, paths.YtDlp)
		}
		return setupDoneMsg{paths: paths}
	}
}

func (m setupModel) Update(msg tea.Msg) (setupModel, tea.Cmd) {
	switch msg := msg.(type) {
	case setupDoneMsg:
		m.done = true
		m.paths = msg.paths
		return m, nil
	case setupErrMsg:
		m.err = msg.err
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m setupModel) View() string {
	if m.err != nil {
		return boxStyle.Render(
			titleStyle.Render("Setup Failed") + "\n\n" +
				errorStyle.Render(fmt.Sprintf("Loi: %v", m.err)) + "\n\n" +
				mutedStyle.Render("Kiem tra ket noi mang va thu lai."),
		)
	}
	if m.done {
		if m.updateOnly {
			return boxStyle.Render(
				successStyle.Render("OK yt-dlp da san sang!") + "\n" +
					mutedStyle.Render("Dang khoi dong..."),
			)
		}
		return boxStyle.Render(
			successStyle.Render("OK Setup hoan tat!") + "\n" +
				mutedStyle.Render("Dang khoi dong..."),
		)
	}
	if m.updateOnly {
		return boxStyle.Render(
			titleStyle.Render("Kiem tra cap nhat") + "\n\n" +
				m.spinner.View() + " Dang kiem tra yt-dlp...\n\n" +
				mutedStyle.Render("Se kiem tra lai toi da moi 24 gio."),
		)
	}
	return boxStyle.Render(
		titleStyle.Render("Cai dat lan dau") + "\n\n" +
			m.spinner.View() + " Dang tai dependencies (yt-dlp, ffmpeg)...\n\n" +
			mutedStyle.Render("Chi can tai 1 lan duy nhat."),
	)
}
