import {useEffect, useState} from 'react'
import {useTranslation} from 'react-i18next'
import Button from '@mui/material/Button'
import DialogContentText from '@mui/material/DialogContentText'
import TextField from '@mui/material/TextField'
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
                    <Button onClick={skip}>{t('loginDialog.skip')}</Button>
                    <Button
                        variant="contained"
                        onClick={signIn}
                        disabled={username.length === 0 && password.length === 0}
                    >
                        {t('loginDialog.signIn')}
                    </Button>
                </>
            }
        >
            <DialogContentText color="text.secondary" sx={{whiteSpace: 'pre-line'}}>
                {t('loginDialog.message', {url: prompt.url})}
            </DialogContentText>
            <TextField
                size="small"
                placeholder={t('loginDialog.username') ?? ''}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
            />
            <TextField
                size="small"
                type="password"
                placeholder={t('loginDialog.password') ?? ''}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
            />
        </Modal>
    )
}
