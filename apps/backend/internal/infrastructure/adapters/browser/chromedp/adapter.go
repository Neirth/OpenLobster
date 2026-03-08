package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/chromedp/chromedp"
	"github.com/neirth/openlobster/internal/domain/ports"
)

type ChromeDPAdapter struct {
	allocCtx    context.Context
	cancelAlloc context.CancelFunc
	pages       map[string]*ChromePage
	mu          sync.RWMutex
}

type ChromeDPConfig struct {
	Headless    bool
	UserAgent   string
	WindowSize  string
	ProxyServer string
}

func NewChromeDPAdapter(config ChromeDPConfig) *ChromeDPAdapter {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	}

	if config.Headless {
		opts = append(opts, chromedp.Headless)
	}

	// Required for containerized and restricted environments (Docker, CI, rootless).
	opts = append(opts,
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
	)

	if config.WindowSize != "" {
		opts = append(opts, chromedp.WindowSize(parseWindowSize(config.WindowSize)))
	}

	if config.ProxyServer != "" {
		opts = append(opts, chromedp.ProxyServer(config.ProxyServer))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)

	adapter := &ChromeDPAdapter{
		allocCtx:    allocCtx,
		cancelAlloc: cancelAlloc,
		pages:       make(map[string]*ChromePage),
	}

	return adapter
}

func parseWindowSize(size string) (int, int) {
	var w, h int
	fmt.Sscanf(size, "%dx%d", &w, &h)
	if w == 0 {
		w = 1920
	}
	if h == 0 {
		h = 1080
	}
	return w, h
}

func (a *ChromeDPAdapter) NewPage(ctx context.Context) (ports.BrowserPage, error) {
	browserCtx, cancel := chromedp.NewContext(a.allocCtx)
	page := &ChromePage{
		ctx:    browserCtx,
		cancel: cancel,
		id:     fmt.Sprintf("page-%d", len(a.pages)),
	}

	a.mu.Lock()
	a.pages[page.id] = page
	a.mu.Unlock()

	return page, nil
}

func (a *ChromeDPAdapter) Close() error {
	a.cancelAlloc()
	return nil
}

type ChromePage struct {
	ctx    context.Context
	cancel context.CancelFunc
	id     string
	url    string
	title  string
}

func (p *ChromePage) Navigate(ctx context.Context, url string) error {
	p.url = url
	var err error
	p.title, err = p.getTitle(ctx, url)
	return err
}

func (p *ChromePage) getTitle(ctx context.Context, url string) (string, error) {
	var title string
	err := chromedp.Run(p.ctx,
		chromedp.Navigate(url),
		chromedp.Title(&title),
	)
	return title, err
}

func (p *ChromePage) Screenshot(ctx context.Context) ([]byte, error) {
	var screenshot []byte
	err := chromedp.Run(p.ctx, chromedp.FullScreenshot(&screenshot, 100))
	return screenshot, err
}

func (p *ChromePage) Click(ctx context.Context, selector string) error {
	return chromedp.Run(p.ctx,
		chromedp.Click(selector),
	)
}

func (p *ChromePage) Type(ctx context.Context, selector, text string) error {
	return chromedp.Run(p.ctx,
		chromedp.SetValue(selector, text),
	)
}

func (p *ChromePage) Eval(ctx context.Context, script string) (interface{}, error) {
	var result interface{}
	err := chromedp.Run(p.ctx,
		chromedp.Evaluate(script, &result),
	)
	return result, err
}

func (p *ChromePage) WaitForSelector(ctx context.Context, selector string) error {
	return chromedp.Run(p.ctx,
		chromedp.WaitVisible(selector),
	)
}

func (p *ChromePage) Close() error {
	p.cancel()
	return nil
}

var _ ports.BrowserPort = (*ChromeDPAdapter)(nil)
var _ ports.BrowserPage = (*ChromePage)(nil)
