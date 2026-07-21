import {useEffect, useState} from 'react'
import {useTranslation} from 'react-i18next'
import {SkipAuthentication, SubmitPassword} from '../../wailsjs/go/main/App'
import type {AuthPrompt} from '../types'
import Modal from './Modal'

interface Props {
    prompt: AuthPrompt
    onResolved: () => void
}

export default function PasswordDialog({prompt, onResolved}: Props) {
    const {t} = useTranslation()
    const [password, setPassword] = useState('')

    useEffect(() => {
        setPassword('')
    }, [prompt.jobId])

    const skip = () => {
        SkipAuthentication(prompt.jobId)
        onResolved()
    }
    const submit = () => {
        SubmitPassword(prompt.jobId, password)
        onResolved()
    }

    return (
        <Modal
            title={t('passwordDialog.title')}
            actions={
                <>
                    <button className="btn" onClick={skip}>{t('passwordDialog.skip')}</button>
                    <button className="btn btn-primary" onClick={submit} disabled={password.length === 0}>
                        {t('passwordDialog.submit')}
                    </button>
                </>
            }
        >
            <p className="modal-text modal-text-muted">
                {t('passwordDialog.message', {url: prompt.url})}
            </p>
            <input
                className="text-field"
                type="password"
                placeholder={t('passwordDialog.password') ?? ''}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoFocus
            />
        </Modal>
    )
}
