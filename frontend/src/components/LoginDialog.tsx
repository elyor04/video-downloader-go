import {useEffect, useState} from 'react'
import {useTranslation} from 'react-i18next'
import {SkipAuthentication, SubmitLogin} from '../../wailsjs/go/main/App'
import type {AuthPrompt} from '../types'
import Modal from './Modal'

interface Props {
    prompt: AuthPrompt
    onResolved: () => void
}

export default function LoginDialog({prompt, onResolved}: Props) {
    const {t} = useTranslation()
    const [username, setUsername] = useState('')
    const [password, setPassword] = useState('')

    useEffect(() => {
        setUsername('')
        setPassword('')
    }, [prompt.jobId])

    const skip = () => {
        SkipAuthentication(prompt.jobId)
        onResolved()
    }
    const signIn = () => {
        SubmitLogin(prompt.jobId, username, password)
        onResolved()
    }

    return (
        <Modal
            title={t('loginDialog.title')}
            actions={
                <>
                    <button className="btn" onClick={skip}>{t('loginDialog.skip')}</button>
                    <button
                        className="btn btn-primary"
                        onClick={signIn}
                        disabled={username.length === 0 && password.length === 0}
                    >
                        {t('loginDialog.signIn')}
                    </button>
                </>
            }
        >
            <p className="modal-text modal-text-muted">
                {t('loginDialog.message', {url: prompt.url})}
            </p>
            <input
                className="text-field"
                placeholder={t('loginDialog.username') ?? ''}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
            />
            <input
                className="text-field"
                type="password"
                placeholder={t('loginDialog.password') ?? ''}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
            />
        </Modal>
    )
}
