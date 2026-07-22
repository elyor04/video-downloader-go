import {useTranslation} from 'react-i18next'
import Button from '@mui/material/Button'
import DialogContentText from '@mui/material/DialogContentText'
import Modal from './Modal'

interface Props {
    message: string
    onClose: () => void
}

export default function ErrorDialog({message, onClose}: Props) {
    const {t} = useTranslation()
    // Backend-originated validation errors are sent as i18n keys prefixed
    // with "error." (see utils.CheckDownloadDir / manager.AddJob); anything
    // else is already-final display text (yt-dlp/OS error strings) and must
    // be shown verbatim rather than run through t().
    const displayMessage = message.startsWith('error.') ? t(message) : message
    return (
        <Modal title={t('app.error')} actions={
            <Button variant="contained" onClick={onClose} autoFocus>{t('app.ok')}</Button>
        }>
            <DialogContentText color="error" sx={{whiteSpace: 'pre-line'}}>
                {displayMessage}
            </DialogContentText>
        </Modal>
    )
}
