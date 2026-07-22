package main

import "video-downloader-go/internal/manager"

// App is the only struct bound to Wails. It delegates to manager.Manager,
// exposing just the methods the frontend should be able to call — unlike
// binding *manager.Manager directly, which would also expose its
// SetEmitter/SetBrowseDirFunc wiring methods (Go-only setup, called once
// from main.go's OnStartup) to JavaScript.
type App struct {
	mgr *manager.Manager
}

func NewApp(mgr *manager.Manager) *App {
	return &App{mgr: mgr}
}

func (a *App) GetInitialState() manager.InitialState { return a.mgr.GetInitialState() }

func (a *App) SetMode(mode string)           { a.mgr.SetMode(mode) }
func (a *App) SetResolution(value int)       { a.mgr.SetResolution(value) }
func (a *App) ResetResolutionToBest()        { a.mgr.ResetResolutionToBest() }
func (a *App) SetConvertTo(value string)     { a.mgr.SetConvertTo(value) }
func (a *App) ResetConvertToOriginal()       { a.mgr.ResetConvertToOriginal() }
func (a *App) SetOutputDir(path string)      { a.mgr.SetOutputDir(path) }
func (a *App) BrowseOutputDir() string       { return a.mgr.BrowseOutputDir() }
func (a *App) SetFileName(value string)      { a.mgr.SetFileName(value) }
func (a *App) SetLanguage(code string)       { a.mgr.SetLanguage(code) }
func (a *App) SetWindowFocused(focused bool) { a.mgr.SetWindowFocused(focused) }

func (a *App) RequestPreview(url string) { a.mgr.RequestPreview(url) }

func (a *App) AddJob(url string) { a.mgr.AddJob(url) }
func (a *App) ConfirmPlaylist(jobID string, downloadAll bool) {
	a.mgr.ConfirmPlaylist(jobID, downloadAll)
}
func (a *App) SubmitLogin(jobID, username, password string) {
	a.mgr.SubmitLogin(jobID, username, password)
}
func (a *App) SubmitPassword(jobID, password string) { a.mgr.SubmitPassword(jobID, password) }
func (a *App) SkipAuthentication(jobID string)       { a.mgr.SkipAuthentication(jobID) }
func (a *App) CancelJob(jobID string)                { a.mgr.CancelJob(jobID) }
func (a *App) RemoveJob(jobID string)                { a.mgr.RemoveJob(jobID) }
func (a *App) CancelAll()                            { a.mgr.CancelAll() }
func (a *App) ClearCompleted()                       { a.mgr.ClearCompleted() }
func (a *App) OpenOutputFolder(jobID string)         { a.mgr.OpenOutputFolder(jobID) }

func (a *App) ConfirmUpdate(kind string, accept bool) error { return a.mgr.ConfirmUpdate(kind, accept) }
