import {useEffect, useState} from 'react'
import {useTranslation} from 'react-i18next'
import Button from '@mui/material/Button'
import DialogContentText from '@mui/material/DialogContentText'
import TextField from '@mui/material/TextField'
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
        if (password.length === 0) return
        SubmitPassword(prompt.jobId, password)
        onResolved()
    }

    return (
        <Modal
            title={t('passwordDialog.title')}
            actions={
                <>
                    <Button onClick={skip}>{t('passwordDialog.skip')}</Button>
                    <Button variant="contained" onClick={submit} disabled={password.length === 0}>
                        {t('passwordDialog.submit')}
                    </Button>
                </>
            }
        >
            <DialogContentText color="text.secondary" sx={{whiteSpace: 'pre-line'}}>
                {t('passwordDialog.message', {url: prompt.url})}
            </DialogContentText>
            <TextField
                size="small"
                type="password"
                placeholder={t('passwordDialog.password') ?? ''}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter') submit()
                }}
                autoFocus
            />
        </Modal>
    )
}
