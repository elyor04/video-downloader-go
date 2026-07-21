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
    return (
        <Modal title={t('app.error')} actions={
            <Button variant="contained" onClick={onClose} autoFocus>{t('app.ok')}</Button>
        }>
            <DialogContentText color="error" sx={{whiteSpace: 'pre-line'}}>
                {message}
            </DialogContentText>
        </Modal>
    )
}
