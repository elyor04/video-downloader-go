import {useTranslation} from 'react-i18next'
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
        <div className="job-row">
            <div className="job-thumb">
                {job.thumbnail ? (
                    <img src={job.thumbnail} alt="" />
                ) : (
                    <span className="job-thumb-fallback">{job.mode === 'audio' ? '♪' : '▶'}</span>
                )}
            </div>

            <div className="job-info">
                <div className="job-title" title={job.title}>{job.title}</div>
                <div className="progress-track">
                    <div
                        className={'progress-fill' + (indeterminate ? ' indeterminate' : '')}
                        style={indeterminate ? undefined : {width: `${Math.max(0, Math.min(1, job.progress)) * 100}%`}}
                    />
                </div>
                <div className="job-status">{statusLine(t, job)}</div>
            </div>

            <div className="job-actions">
                {job.canOpenFolder && (
                    <button className="btn-link" onClick={() => OpenOutputFolder(job.jobId)}>
                        {t('job.openFolder')}
                    </button>
                )}
                {job.canCancel && (
                    <button className="btn-link" onClick={() => CancelJob(job.jobId)}>
                        {t('job.cancel')}
                    </button>
                )}
                {job.canRemove && (
                    <button className="btn-link" onClick={() => RemoveJob(job.jobId)}>
                        {t('job.remove')}
                    </button>
                )}
            </div>
        </div>
    )
}
