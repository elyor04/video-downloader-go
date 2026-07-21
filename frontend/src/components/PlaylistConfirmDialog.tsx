import {useTranslation} from 'react-i18next'
import {ConfirmPlaylist} from '../../wailsjs/go/main/App'
import type {PlaylistPrompt} from '../types'
import Modal from './Modal'

interface Props {
    prompt: PlaylistPrompt
    onResolved: () => void
}

export default function PlaylistConfirmDialog({prompt, onResolved}: Props) {
    const {t} = useTranslation()

    const respond = (downloadAll: boolean) => {
        ConfirmPlaylist(prompt.jobId, downloadAll)
        onResolved()
    }

    return (
        <Modal
            title={t('playlistDialog.title')}
            actions={
                <>
                    <button className="btn" onClick={() => respond(false)}>{t('playlistDialog.no')}</button>
                    <button className="btn btn-primary" onClick={() => respond(true)} autoFocus>
                        {t('playlistDialog.yes')}
                    </button>
                </>
            }
        >
            <p className="modal-text">
                {prompt.count > 0
                    ? t('playlistDialog.withCount', {count: prompt.count})
                    : t('playlistDialog.withoutCount')}
            </p>
        </Modal>
    )
}
