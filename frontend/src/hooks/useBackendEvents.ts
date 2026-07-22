import {useEffect, useState} from 'react'
import {EventsOn} from '../../wailsjs/runtime/runtime'
import {GetInitialState, SetWindowFocused} from '../../wailsjs/go/main/App'
import type {
    AuthPrompt,
    JobDTO,
    OptionsState,
    PlaylistPrompt,
    PreviewState,
    ResolutionOption,
    UpdatePrompt,
} from '../types'

const idlePreview: PreviewState = {
    url: '',
    state: 'idle',
    title: '',
    thumbnail: '',
    isPlaylist: false,
    playlistCount: 0,
    error: '',
}

export interface BackendState {
    ready: boolean
    mode: string
    resolution: number
    convertTo: string
    outputDir: string
    language: string
    resolutionOptions: ResolutionOption[]
    convertOptions: string[]
    jobs: JobDTO[]
    preview: PreviewState
    errorMessage: string | null
    playlistPrompt: PlaylistPrompt | null
    loginPrompt: AuthPrompt | null
    passwordPrompt: AuthPrompt | null
    ytdlpUpdatePrompt: UpdatePrompt | null
    ffmpegUpdatePrompt: UpdatePrompt | null
}

const initialState: BackendState = {
    ready: false,
    mode: 'video',
    resolution: 0,
    convertTo: 'original',
    outputDir: '',
    language: 'en',
    resolutionOptions: [],
    convertOptions: [],
    jobs: [],
    preview: idlePreview,
    errorMessage: null,
    playlistPrompt: null,
    loginPrompt: null,
    passwordPrompt: null,
    ytdlpUpdatePrompt: null,
    ffmpegUpdatePrompt: null,
}

// useBackendEvents is the single point of contact with the Go backend: it
// loads the startup snapshot once, then keeps state in sync with every
// event the manager emits (replacing the Qt Signal/Property bindings QML
// used to react to directly).
export function useBackendEvents() {
    const [state, setState] = useState<BackendState>(initialState)

    useEffect(() => {
        GetInitialState().then((initial) => {
            setState((s) => ({
                ...s,
                ready: true,
                mode: initial.mode,
                resolution: initial.resolution,
                convertTo: initial.convertTo,
                outputDir: initial.outputDir,
                language: initial.language,
                resolutionOptions: initial.resolutionOptions,
                convertOptions: initial.convertOptions,
                jobs: initial.jobs,
            }))
        })

        const offs = [
            EventsOn('preview:changed', (p: PreviewState) =>
                setState((s) => ({...s, preview: p}))),
            EventsOn('resolution-options:changed', (opts: ResolutionOption[]) =>
                setState((s) => ({...s, resolutionOptions: opts}))),
            EventsOn('options:changed', (o: OptionsState) =>
                setState((s) => ({
                    ...s,
                    mode: o.mode,
                    resolution: o.resolution,
                    convertTo: o.convertTo,
                    convertOptions: o.convertOptions,
                }))),
            EventsOn('output-dir:changed', (dir: string) =>
                setState((s) => ({...s, outputDir: dir}))),
            EventsOn('language:changed', (lang: string) =>
                setState((s) => ({...s, language: lang}))),
            EventsOn('job:added', (j: JobDTO) =>
                setState((s) => ({...s, jobs: [...s.jobs, j]}))),
            EventsOn('job:updated', (j: JobDTO) =>
                setState((s) => ({
                    ...s,
                    jobs: s.jobs.map((x) => (x.jobId === j.jobId ? j : x)),
                }))),
            EventsOn('job:removed', (id: string) =>
                setState((s) => ({...s, jobs: s.jobs.filter((x) => x.jobId !== id)}))),
            EventsOn('playlist:detected', (p: PlaylistPrompt) =>
                setState((s) => ({...s, playlistPrompt: p}))),
            EventsOn('login:requested', (p: AuthPrompt) =>
                setState((s) => ({...s, loginPrompt: p}))),
            EventsOn('password:requested', (p: AuthPrompt) =>
                setState((s) => ({...s, passwordPrompt: p}))),
            EventsOn('update:ytdlp-available', (p: UpdatePrompt) =>
                setState((s) => ({...s, ytdlpUpdatePrompt: p}))),
            EventsOn('update:ffmpeg-available', (p: UpdatePrompt) =>
                setState((s) => ({...s, ffmpegUpdatePrompt: p}))),
            EventsOn('prompt:cancelled', (jobId: string) =>
                setState((s) => ({
                    ...s,
                    playlistPrompt: s.playlistPrompt?.jobId === jobId ? null : s.playlistPrompt,
                    loginPrompt: s.loginPrompt?.jobId === jobId ? null : s.loginPrompt,
                    passwordPrompt: s.passwordPrompt?.jobId === jobId ? null : s.passwordPrompt,
                }))),
            EventsOn('error:occurred', (message: string) =>
                setState((s) => ({...s, errorMessage: message}))),
        ]

        return () => offs.forEach((off) => off())
    }, [])

    // Mirrors _notify_if_unfocused's applicationState() check: the backend
    // only fires a native notification while the window isn't focused.
    useEffect(() => {
        const onFocus = () => SetWindowFocused(true)
        const onBlur = () => SetWindowFocused(false)
        window.addEventListener('focus', onFocus)
        window.addEventListener('blur', onBlur)
        SetWindowFocused(document.hasFocus())
        return () => {
            window.removeEventListener('focus', onFocus)
            window.removeEventListener('blur', onBlur)
        }
    }, [])

    return {
        state,
        dismissError: () => setState((s) => ({...s, errorMessage: null})),
        dismissPlaylistPrompt: () => setState((s) => ({...s, playlistPrompt: null})),
        dismissLoginPrompt: () => setState((s) => ({...s, loginPrompt: null})),
        dismissPasswordPrompt: () => setState((s) => ({...s, passwordPrompt: null})),
        dismissYtdlpUpdatePrompt: () => setState((s) => ({...s, ytdlpUpdatePrompt: null})),
        dismissFfmpegUpdatePrompt: () => setState((s) => ({...s, ffmpegUpdatePrompt: null})),
    }
}
