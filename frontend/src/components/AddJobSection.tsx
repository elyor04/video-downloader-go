import {useEffect, useRef, useState} from 'react'
import {useTranslation} from 'react-i18next'
import Avatar from '@mui/material/Avatar'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import FormControlLabel from '@mui/material/FormControlLabel'
import MenuItem from '@mui/material/MenuItem'
import Paper from '@mui/material/Paper'
import Radio from '@mui/material/Radio'
import Select from '@mui/material/Select'
import Stack from '@mui/material/Stack'
import TextField from '@mui/material/TextField'
import Typography from '@mui/material/Typography'
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
        <Paper variant="outlined" sx={{display: 'flex', flexDirection: 'column', gap: 1.25, p: 2}}>
            <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                <TextField
                    size="small"
                    sx={{flex: 1}}
                    placeholder={t('addJobSection.urlPlaceholder') ?? ''}
                    value={url}
                    onChange={(e) => setUrl(e.target.value)}
                    onKeyDown={(e) => {
                        if (e.key === 'Enter' && canDownload) startDownload()
                    }}
                />
                <FormControlLabel
                    control={<Radio size="small" checked={mode === 'video'} onChange={() => changeMode('video')}/>}
                    label={t('addJobSection.video')}
                    sx={{whiteSpace: 'nowrap', mr: 0}}
                />
                <FormControlLabel
                    control={<Radio size="small" checked={mode === 'audio'} onChange={() => changeMode('audio')}/>}
                    label={t('addJobSection.audio')}
                    sx={{whiteSpace: 'nowrap', mr: 0}}
                />
            </Stack>

            {state.preview.state !== 'idle' && (
                <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    {state.preview.state === 'fetching' && <CircularProgress size={18}/>}
                    {state.preview.state === 'ready' && state.preview.thumbnail && (
                        <Avatar variant="rounded" src={state.preview.thumbnail} sx={{width: 36, height: 36}}/>
                    )}
                    <Typography
                        variant="caption"
                        color={state.preview.state === 'error' ? 'error' : 'text.secondary'}
                        noWrap
                        sx={{flex: 1}}
                    >
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
                    </Typography>
                </Stack>
            )}

            <Stack direction="row" spacing={1}>
                {mode === 'video' && (
                    <Select
                        size="small"
                        value={resolution}
                        sx={{minWidth: 160}}
                        onChange={(e) => {
                            const value = Number(e.target.value)
                            setLocalResolution(value)
                            SetResolution(value)
                        }}
                    >
                        {state.resolutionOptions.map((opt) => (
                            <MenuItem key={opt.value} value={opt.value}>
                                {resolutionLabel(opt.label)}
                            </MenuItem>
                        ))}
                    </Select>
                )}
                <Select
                    size="small"
                    value={convertTo}
                    sx={{minWidth: 140}}
                    onChange={(e) => {
                        setLocalConvertTo(e.target.value)
                        SetConvertTo(e.target.value)
                    }}
                >
                    {state.convertOptions.map((opt) => (
                        <MenuItem key={opt} value={opt}>
                            {convertLabel(opt)}
                        </MenuItem>
                    ))}
                </Select>
                <TextField
                    size="small"
                    sx={{flex: 1}}
                    placeholder={t('addJobSection.fileNamePlaceholder') ?? ''}
                    onChange={(e) => SetFileName(e.target.value)}
                />
            </Stack>

            <Stack direction="row" spacing={1}>
                <TextField size="small" sx={{flex: 1}} slotProps={{input: {readOnly: true}}} value={state.outputDir}/>
                <Button variant="outlined" onClick={() => BrowseOutputDir()}>
                    {t('addJobSection.browse')}
                </Button>
            </Stack>

            <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
                <Button variant="contained" disabled={!canDownload} onClick={startDownload}>
                    {t('addJobSection.download')}
                </Button>
            </Box>
        </Paper>
    )
}
