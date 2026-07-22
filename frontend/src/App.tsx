import type {ReactNode} from 'react'
import {useTranslation} from 'react-i18next'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import MenuItem from '@mui/material/MenuItem'
import Select from '@mui/material/Select'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import {CancelAll, ClearCompleted, SetLanguage} from '../wailsjs/go/main/App'
import AddJobSection from './components/AddJobSection'
import ErrorDialog from './components/ErrorDialog'
import LoginDialog from './components/LoginDialog'
import PasswordDialog from './components/PasswordDialog'
import PlaylistConfirmDialog from './components/PlaylistConfirmDialog'
import QueueList from './components/QueueList'
import TitleBar from './components/TitleBar'
import UpdateDialog from './components/UpdateDialog'
import {useBackendEvents} from './hooks/useBackendEvents'

const LANGUAGES: Array<{code: string; labelKey: string}> = [
    {code: 'en', labelKey: 'language.en'},
    {code: 'ru', labelKey: 'language.ru'},
    {code: 'uz', labelKey: 'language.uz'},
]

export default function App() {
    const {t, i18n} = useTranslation()
    const {
        state,
        reportError,
        dismissError,
        dismissPlaylistPrompt,
        dismissLoginPrompt,
        dismissPasswordPrompt,
        dismissYtdlpUpdatePrompt,
        dismissFfmpegUpdatePrompt,
    } = useBackendEvents()

    const changeLanguage = (code: string) => {
        void i18n.changeLanguage(code)
        SetLanguage(code)
    }

    const hasCancellableJobs = state.jobs.some((j) => j.canCancel)

    // Only one modal is ever shown at a time -- state.errorMessage,
    // playlist/login/password prompts, and the update prompts are all
    // independent state slices with no shared coordination, so without an
    // explicit precedence order here two could render stacked (e.g. an
    // update failure's error dialog on top of a still-pending ffmpeg update
    // prompt). Live per-job prompts block that job's running download, so
    // they outrank the app-level update prompts, which in turn outrank the
    // purely informational error dialog.
    let modal: ReactNode = null
    if (state.playlistPrompt !== null) {
        modal = <PlaylistConfirmDialog prompt={state.playlistPrompt} onResolved={dismissPlaylistPrompt}/>
    } else if (state.loginPrompt !== null) {
        modal = <LoginDialog prompt={state.loginPrompt} onResolved={dismissLoginPrompt}/>
    } else if (state.passwordPrompt !== null) {
        modal = <PasswordDialog prompt={state.passwordPrompt} onResolved={dismissPasswordPrompt}/>
    } else if (state.ytdlpUpdatePrompt !== null) {
        modal = <UpdateDialog prompt={state.ytdlpUpdatePrompt} onResolved={dismissYtdlpUpdatePrompt} onError={reportError}/>
    } else if (state.ffmpegUpdatePrompt !== null) {
        modal = <UpdateDialog prompt={state.ffmpegUpdatePrompt} onResolved={dismissFfmpegUpdatePrompt} onError={reportError}/>
    } else if (state.errorMessage !== null) {
        modal = <ErrorDialog message={state.errorMessage} onClose={dismissError}/>
    }

    return (
        <Box sx={{display: 'flex', flexDirection: 'column', height: '100vh', overflow: 'hidden'}}>
            <TitleBar/>

            <Box sx={{display: 'flex', flexDirection: 'column', gap: 1.5, p: 1.5, flex: '1 1 auto', minHeight: 0}}>
                <Stack direction="row" sx={{alignItems: 'center'}}>
                    <Typography sx={{fontSize: 20, fontWeight: 700, flex: 1}}>
                        {t('app.title')}
                    </Typography>
                    <Select
                        size="small"
                        value={i18n.language}
                        onChange={(e) => changeLanguage(e.target.value)}
                        sx={{minWidth: 140}}
                    >
                        {LANGUAGES.map((lang) => (
                            <MenuItem key={lang.code} value={lang.code}>
                                {t(lang.labelKey)}
                            </MenuItem>
                        ))}
                    </Select>
                </Stack>

                <AddJobSection state={state}/>

                <Stack direction="row" sx={{justifyContent: 'space-between', alignItems: 'center'}}>
                    <Typography variant="subtitle1" sx={{fontWeight: 700}}>
                        {t('app.downloads')}
                    </Typography>
                    <Stack direction="row" spacing={1}>
                        <Button size="small" color="error" disabled={!hasCancellableJobs} onClick={() => CancelAll()}>
                            {t('app.cancelAll')}
                        </Button>
                        <Button size="small" onClick={() => ClearCompleted()}>
                            {t('app.clearCompleted')}
                        </Button>
                    </Stack>
                </Stack>

                <QueueList jobs={state.jobs}/>
            </Box>

            {modal}
        </Box>
    )
}
