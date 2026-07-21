import {Component, type ErrorInfo, type ReactNode} from 'react'
import Alert from '@mui/material/Alert'
import AlertTitle from '@mui/material/AlertTitle'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Stack from '@mui/material/Stack'
import Typography from '@mui/material/Typography'

interface Props {
    children: ReactNode
}

interface State {
    error: Error | null
}

export default class ErrorBoundary extends Component<Props, State> {
    state: State = {error: null}

    static getDerivedStateFromError(error: Error): State {
        return {error}
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('Unhandled render error', error, info)
    }

    render() {
        const {error} = this.state
        if (!error) return this.props.children

        return (
            <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh', p: 3}}>
                <Stack spacing={2} sx={{maxWidth: 420}}>
                    <Alert severity="error" variant="filled">
                        <AlertTitle>Something went wrong</AlertTitle>
                        <Typography variant="body2" sx={{wordBreak: 'break-word'}}>
                            {error.message}
                        </Typography>
                    </Alert>
                    <Button variant="contained" onClick={() => window.location.reload()}>
                        Reload
                    </Button>
                </Stack>
            </Box>
        )
    }
}
