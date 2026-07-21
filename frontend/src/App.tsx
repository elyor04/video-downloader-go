import {useTranslation} from 'react-i18next'
import {ClearCompleted, SetLanguage} from '../wailsjs/go/main/App'
import AddJobSection from './components/AddJobSection'
import ErrorDialog from './components/ErrorDialog'
import LoginDialog from './components/LoginDialog'
import PasswordDialog from './components/PasswordDialog'
import PlaylistConfirmDialog from './components/PlaylistConfirmDialog'
import QueueList from './components/QueueList'
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
    } = useBackendEvents()

    const changeLanguage = (code: string) => {
        void i18n.changeLanguage(code)
        SetLanguage(code)
    }

    return (
        <div className="app">
            <header className="app-header">
                <h1>{t('app.title')}</h1>
                <select
                    className="select language-select"
                    value={i18n.language}
                    onChange={(e) => changeLanguage(e.target.value)}
                >
                    {LANGUAGES.map((lang) => (
                        <option key={lang.code} value={lang.code}>
                            {t(lang.labelKey)}
                        </option>
                    ))}
                </select>
            </header>

            <AddJobSection state={state} />

            <div className="row downloads-header">
                <h2>{t('app.downloads')}</h2>
                <button className="btn-link" onClick={() => ClearCompleted()}>
                    {t('app.clearCompleted')}
                </button>
            </div>

            <QueueList jobs={state.jobs} />

            {state.errorMessage !== null && (
                <ErrorDialog message={state.errorMessage} onClose={dismissError} />
            )}
            {state.playlistPrompt !== null && (
                <PlaylistConfirmDialog prompt={state.playlistPrompt} onResolved={dismissPlaylistPrompt} />
            )}
            {state.loginPrompt !== null && (
                <LoginDialog prompt={state.loginPrompt} onResolved={dismissLoginPrompt} />
            )}
            {state.passwordPrompt !== null && (
                <PasswordDialog prompt={state.passwordPrompt} onResolved={dismissPasswordPrompt} />
            )}
        </div>
    )
}
