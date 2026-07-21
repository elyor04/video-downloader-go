import React from 'react'
import {createRoot} from 'react-dom/client'
import CssBaseline from '@mui/material/CssBaseline'
import {ThemeProvider} from '@mui/material/styles'
import './i18n'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import theme from './theme'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ThemeProvider theme={theme}>
            <CssBaseline/>
            <ErrorBoundary>
                <App/>
            </ErrorBoundary>
        </ThemeProvider>
    </React.StrictMode>
)
