import {useTranslation} from 'react-i18next'
import type {JobDTO} from '../types'
import JobRow from './JobRow'

export default function QueueList({jobs}: {jobs: JobDTO[]}) {
    const {t} = useTranslation()

    if (jobs.length === 0) {
        return <div className="queue-empty">{t('queueList.empty')}</div>
    }

    return (
        <div className="queue-list">
            {jobs.map((job) => (
                <JobRow key={job.jobId} job={job} />
            ))}
        </div>
    )
}
