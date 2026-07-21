import {useTranslation} from 'react-i18next'
import Avatar from '@mui/material/Avatar'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Chip from '@mui/material/Chip'
import LinearProgress from '@mui/material/LinearProgress'
import Paper from '@mui/material/Paper'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import MusicNoteIcon from '@mui/icons-material/MusicNote'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import {CancelJob, OpenOutputFolder, RemoveJob} from '../../wailsjs/go/main/App'
import type {JobDTO} from '../types'

function statusLine(t: (key: string, opts?: Record<string, unknown>) => string, job: JobDTO): string {
    switch (job.jobState) {
        case 'fetching':
            return t('job.fetching')
        case 'awaiting_playlist_confirm':
            return t('job.awaitingPlaylistConfirm')
        case 'awaiting_login':
            return t('job.awaitingLogin')
        case 'awaiting_password':
            return t('job.awaitingPassword')
        case 'queued':
            return t('job.queued')
        case 'downloading':
            if (job.stage === 'converting') {
                return t('job.converting')
            }
            if (job.playlistIndex > 0 && job.playlistCount > 0) {
                return t('job.downloadingPlaylist', {
                    index: job.playlistIndex,
                    count: job.playlistCount,
                    downloaded: job.downloadedText,
                    total: job.totalText,
                    speed: job.speedText,
                    eta: job.etaText,
                })
            }
            return t('job.downloading', {
                downloaded: job.downloadedText,
                total: job.totalText,
                speed: job.speedText,
                eta: job.etaText,
            })
        case 'success':
            return t('job.done')
        case 'error':
            return t('job.error', {message: job.errorMessage})
        case 'cancelled':
            return t('job.cancelled')
        default:
            return ''
    }
}

export default function JobRow({job}: {job: JobDTO}) {
    const {t} = useTranslation()
    const indeterminate = job.progress < 0 && job.jobState !== 'error' && job.jobState !== 'cancelled'

    return (
        <Paper variant="outlined" sx={{display: 'flex', alignItems: 'center', gap: 1.5, p: 1.5}}>
            <Avatar
                variant="rounded"
                src={job.thumbnail || undefined}
                sx={{width: 64, height: 64, flex: '0 0 auto', bgcolor: 'action.selected', color: 'text.secondary'}}
            >
                {job.mode === 'audio' ? <MusicNoteIcon/> : <PlayArrowIcon/>}
            </Avatar>

            <Box sx={{flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.5}}>
                <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Typography variant="subtitle2" noWrap title={job.title} sx={{flex: 1, fontWeight: 700}}>
                        {job.title}
                    </Typography>
                    {job.jobState === 'success' && <Chip size="small" color="success" label={t('job.done')}/>}
                    {job.jobState === 'error' && <Chip size="small" color="error" label={t('app.error')}/>}
                </Stack>
                <LinearProgress
                    variant={indeterminate ? 'indeterminate' : 'determinate'}
                    value={indeterminate ? undefined : Math.max(0, Math.min(1, job.progress)) * 100}
                />
                <Typography variant="caption" color="text.secondary" noWrap>
                    {statusLine(t, job)}
                </Typography>
            </Box>

            <Stack direction="row" spacing={0.5} sx={{flex: '0 0 auto'}}>
                {job.canOpenFolder && (
                    <Button size="small" color="primary" onClick={() => OpenOutputFolder(job.jobId)}>
                        {t('job.openFolder')}
                    </Button>
                )}
                {job.canCancel && (
                    <Button size="small" color="error" onClick={() => CancelJob(job.jobId)}>
                        {t('job.cancel')}
                    </Button>
                )}
                {job.canRemove && (
                    <Button size="small" color="error" onClick={() => RemoveJob(job.jobId)}>
                        {t('job.remove')}
                    </Button>
                )}
            </Stack>
        </Paper>
    )
}
