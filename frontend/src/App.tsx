import {useTranslation} from 'react-i18next'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import MenuItem from '@mui/material/MenuItem'
import Select from '@mui/material/Select'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'
import {ClearCompleted, SetLanguage} from '../wailsjs/go/main/App'
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
                    <Button size="small" onClick={() => ClearCompleted()}>
                        {t('app.clearCompleted')}
                    </Button>
                </Stack>

                <QueueList jobs={state.jobs}/>
            </Box>

            {state.errorMessage !== null && (
                <ErrorDialog message={state.errorMessage} onClose={dismissError}/>
            )}
            {state.playlistPrompt !== null && (
                <PlaylistConfirmDialog prompt={state.playlistPrompt} onResolved={dismissPlaylistPrompt}/>
            )}
            {state.loginPrompt !== null && (
                <LoginDialog prompt={state.loginPrompt} onResolved={dismissLoginPrompt}/>
            )}
            {state.passwordPrompt !== null && (
                <PasswordDialog prompt={state.passwordPrompt} onResolved={dismissPasswordPrompt}/>
            )}
            {state.ytdlpUpdatePrompt !== null ? (
                <UpdateDialog prompt={state.ytdlpUpdatePrompt} onResolved={dismissYtdlpUpdatePrompt}/>
            ) : state.ffmpegUpdatePrompt !== null ? (
                <UpdateDialog prompt={state.ffmpegUpdatePrompt} onResolved={dismissFfmpegUpdatePrompt}/>
            ) : null}
        </Box>
    )
}
