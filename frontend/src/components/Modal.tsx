import type {ReactNode} from 'react'

interface ModalProps {
    title: string
    children: ReactNode
    actions: ReactNode
}

export default function Modal({title, children, actions}: ModalProps) {
    return (
        <div className="modal-overlay">
            <div className="modal-box" role="dialog" aria-modal="true">
                <h2 className="modal-title">{title}</h2>
                <div className="modal-body">{children}</div>
                <div className="modal-actions">{actions}</div>
            </div>
        </div>
    )
}
