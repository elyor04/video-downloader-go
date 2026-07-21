import {useTranslation} from 'react-i18next'
import Modal from './Modal'

interface Props {
    message: string
    onClose: () => void
}

export default function ErrorDialog({message, onClose}: Props) {
    const {t} = useTranslation()
    return (
        <Modal title={t('app.error')} actions={
            <button className="btn btn-primary" onClick={onClose} autoFocus>OK</button>
        }>
            <p className="modal-text modal-text-error">{message}</p>
        </Modal>
    )
}
