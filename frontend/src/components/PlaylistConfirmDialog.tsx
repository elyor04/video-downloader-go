import {useTranslation} from 'react-i18next'
import Button from '@mui/material/Button'
import DialogContentText from '@mui/material/DialogContentText'
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
                    <Button onClick={() => respond(false)}>{t('playlistDialog.no')}</Button>
                    <Button variant="contained" onClick={() => respond(true)} autoFocus>
                        {t('playlistDialog.yes')}
                    </Button>
                </>
            }
        >
            <DialogContentText>
                {prompt.count > 0
                    ? t('playlistDialog.withCount', {count: prompt.count})
                    : t('playlistDialog.withoutCount')}
            </DialogContentText>
        </Modal>
    )
}
