import {useEffect, useRef, useState} from 'react'
import {useTranslation} from 'react-i18next'
import {
    AddJob,
    BrowseOutputDir,
    RequestPreview,
    ResetConvertToOriginal,
    ResetResolutionToBest,
    SetConvertTo,
    SetFileName,
    SetMode,
    SetResolution,
} from '../../wailsjs/go/main/App'
import type {BackendState} from '../hooks/useBackendEvents'

const MAX_RESOLUTION = 65535
const URL_RE = /^https?:\/\/\S+/i
const PREVIEW_DEBOUNCE_MS = 600

export default function AddJobSection({state}: {state: BackendState}) {
    const {t} = useTranslation()
    const [url, setUrl] = useState('')
    const [mode, setLocalMode] = useState<'video' | 'audio'>('video')
    const [resolution, setLocalResolution] = useState(MAX_RESOLUTION)
    const [convertTo, setLocalConvertTo] = useState('original')
    const debounceRef = useRef<number | undefined>(undefined)

    // Mirrors AddJobSection.qml's Timer{interval:600} debounce before the
    // live URL preview kicks off.
    useEffect(() => {
        window.clearTimeout(debounceRef.current)
        debounceRef.current = window.setTimeout(() => {
            const trimmed = url.trim()
            RequestPreview(URL_RE.test(trimmed) ? trimmed : '')
        }, PREVIEW_DEBOUNCE_MS)
        return () => window.clearTimeout(debounceRef.current)
    }, [url])

    // Mirrors the QML Connections that reset the resolution selection back
    // to "Best" whenever the option list itself changes underneath it (a
    // preview resolving narrows the ladder to what the video actually has).
    const skipFirstOptionsReset = useRef(true)
    useEffect(() => {
        if (skipFirstOptionsReset.current) {
            skipFirstOptionsReset.current = false
            return
        }
        setLocalResolution(MAX_RESOLUTION)
        ResetResolutionToBest()
    }, [state.resolutionOptions])

    const changeMode = (next: 'video' | 'audio') => {
        setLocalMode(next)
        SetMode(next)
        // Mirrors onModeChanged resetting the convert-to selection: video
        // and audio have disjoint convert targets, so a stale selection
        // from the other mode would build a nonsensical request.
        setLocalConvertTo('original')
        ResetConvertToOriginal()
    }

    const trimmedUrl = url.trim()
    const canDownload =
        trimmedUrl.length > 0 &&
        state.preview.state === 'ready' &&
        state.preview.url === trimmedUrl

    const startDownload = () => {
        if (!canDownload) return
        AddJob(url)
        setUrl('')
    }

    const resolutionLabel = (label: string) => (label === 'resolution.best' ? t('resolution.best') : label)
    const convertLabel = (value: string) => (value === 'original' ? t('convert.original') : value)

    return (
        <div className="card">
            <div className="row">
                <input
                    className="text-field url-field"
                    placeholder={t('addJobSection.urlPlaceholder') ?? ''}
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' && canDownload) startDownload()
                    }}
                />
                <label className="radio">
                    <input type="radio" checked={mode === 'video'} onChange={() => changeMode('video')} />
                    {t('addJobSection.video')}
                </label>
                <label className="radio">
                    <input type="radio" checked={mode === 'audio'} onChange={() => changeMode('audio')} />
                    {t('addJobSection.audio')}
                </label>
            </div>

            {state.preview.state !== 'idle' && (
                <div className="row preview-row">
                    {state.preview.state === 'fetching' && <span className="spinner" />}
                    {state.preview.state === 'ready' && state.preview.thumbnail && (
                        <img className="preview-thumb" src={state.preview.thumbnail} alt="" />
                    )}
                    <span className={'preview-text' + (state.preview.state === 'error' ? ' preview-error' : '')}>
                        {state.preview.state === 'fetching' && t('addJobSection.lookingUpInfo')}
                        {state.preview.state === 'ready' &&
                            (state.preview.isPlaylist
                                ? t('addJobSection.playlistPreview', {
                                      title: state.preview.title,
                                      count: state.preview.playlistCount,
                                  })
                                : state.preview.title)}
                        {state.preview.state === 'error' &&
                            t('addJobSection.couldntLoadInfo', {error: state.preview.error})}
                    </span>
                </div>
            )}

            <div className="row">
                {mode === 'video' && (
                    <select
                        className="select"
                        value={resolution}
                        onChange={(e) => {
                            const value = Number(e.target.value)
                            setLocalResolution(value)
                            SetResolution(value)
                        }}
                    >
                        {state.resolutionOptions.map((opt) => (
                            <option key={opt.value} value={opt.value}>
                                {resolutionLabel(opt.label)}
                            </option>
                        ))}
                    </select>
                )}
                <select
                    className="select"
                    value={convertTo}
                    onChange={(e) => {
                        setLocalConvertTo(e.target.value)
                        SetConvertTo(e.target.value)
                    }}
                >
                    {state.convertOptions.map((opt) => (
                        <option key={opt} value={opt}>
                            {convertLabel(opt)}
                        </option>
                    ))}
                </select>
                <input
                    className="text-field"
                    placeholder={t('addJobSection.fileNamePlaceholder') ?? ''}
                    onChange={(e) => SetFileName(e.target.value)}
                />
            </div>

            <div className="row">
                <input className="text-field" readOnly value={state.outputDir} />
                <button className="btn" onClick={() => BrowseOutputDir()}>
                    {t('addJobSection.browse')}
                </button>
            </div>

            <div className="row row-end">
                <button className="btn btn-primary" disabled={!canDownload} onClick={startDownload}>
                    {t('addJobSection.download')}
                </button>
            </div>
        </div>
    )
}
