import type {ReactNode} from 'react'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'

interface ModalProps {
    title: string
    children: ReactNode
    actions: ReactNode
}

export default function Modal({title, children, actions}: ModalProps) {
    return (
        <Dialog open onClose={() => {}} fullWidth maxWidth="xs">
            <DialogTitle sx={{fontWeight: 700}}>{title}</DialogTitle>
            <DialogContent sx={{display: 'flex', flexDirection: 'column', gap: 1.5}}>{children}</DialogContent>
            <DialogActions>{actions}</DialogActions>
        </Dialog>
    )
}
