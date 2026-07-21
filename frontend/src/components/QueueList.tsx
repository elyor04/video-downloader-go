import {useTranslation} from 'react-i18next'
import Box from '@mui/material/Box'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import type {JobDTO} from '../types'
import JobRow from './JobRow'

export default function QueueList({jobs}: {jobs: JobDTO[]}) {
    const {t} = useTranslation()

    if (jobs.length === 0) {
        return (
            <Box
                sx={{
                    flex: '1 1 auto',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    textAlign: 'center',
                    px: '20%',
                }}
            >
                <Typography color="text.secondary">{t('queueList.empty')}</Typography>
            </Box>
        )
    }

    return (
        <Stack spacing={1} sx={{flex: '1 1 auto', overflowY: 'auto'}}>
            {jobs.map((job) => (
                <JobRow key={job.jobId} job={job}/>
            ))}
        </Stack>
    )
}
