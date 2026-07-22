import {useState} from 'react'
import {useTranslation} from 'react-i18next'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import DialogContentText from '@mui/material/DialogContentText'
import {ConfirmUpdate} from '../../wailsjs/go/main/App'
import type {UpdatePrompt} from '../types'
import Modal from './Modal'

interface Props {
    prompt: UpdatePrompt
    onResolved: () => void
    onError: (message: string) => void
}

export default function UpdateDialog({prompt, onResolved, onError}: Props) {
    const {t} = useTranslation()
    const [updating, setUpdating] = useState(false)

    const decline = () => {
        ConfirmUpdate(prompt.kind, false)
        onResolved()
    }

    const accept = async () => {
        setUpdating(true)
        try {
            await ConfirmUpdate(prompt.kind, true)
            onResolved()
        } catch (err) {
            setUpdating(false)
            onResolved()
            onError(String(err))
        }
    }

    const titleKey = prompt.missing
        ? (prompt.kind === 'ytdlp' ? 'updateDialog.ytdlpMissingTitle' : 'updateDialog.ffmpegMissingTitle')
        : (prompt.kind === 'ytdlp' ? 'updateDialog.ytdlpTitle' : 'updateDialog.ffmpegTitle')
    const bodyKey = prompt.missing
        ? (prompt.kind === 'ytdlp' ? 'updateDialog.ytdlpMissingBody' : 'updateDialog.ffmpegMissingBody')
        : (prompt.kind === 'ytdlp' ? 'updateDialog.ytdlpBody' : 'updateDialog.ffmpegBody')

    return (
        <Modal
            title={t(titleKey)}
            actions={
                <>
                    <Button onClick={decline} disabled={updating}>{t('updateDialog.later')}</Button>
                    <Button variant="contained" onClick={accept} disabled={updating} autoFocus>
                        {updating ? <CircularProgress size={20}/> : t(prompt.missing ? 'updateDialog.download' : 'updateDialog.update')}
                    </Button>
                </>
            }
        >
            <DialogContentText>
                {t(bodyKey, {
                    current: prompt.currentVersion,
                    latest: prompt.latestVersion,
                })}
            </DialogContentText>
        </Modal>
    )
}
