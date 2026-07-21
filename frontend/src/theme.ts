import {createTheme} from '@mui/material/styles'

const FONT_SANS =
    '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif'

export const TITLE_BAR_HEIGHT = 40

const theme = createTheme({
    palette: {
        mode: 'dark',
        background: {
            default: '#121212',
            paper: '#1a1a1a',
        },
        primary: {
            main: '#2196f3',
            light: '#42a5f5',
        },
        error: {
            main: '#e57373',
        },
        success: {
            main: '#4cb782',
        },
        divider: '#2a2a2a',
    },
    shape: {
        borderRadius: 5,
    },
    typography: {
        fontFamily: FONT_SANS,
    },
    components: {
        MuiCssBaseline: {
            styleOverrides: {
                html: {height: '100%'},
                body: {height: '100%'},
                '#root': {height: '100%'},
                '*::-webkit-scrollbar': {
                    width: 10,
                    height: 10,
                },
                '*::-webkit-scrollbar-track': {
                    background: 'transparent',
                },
                '*::-webkit-scrollbar-thumb': {
                    backgroundColor: '#3a3a3a',
                    borderRadius: 999,
                    border: '2px solid #121212',
                },
                '*::-webkit-scrollbar-thumb:hover': {
                    backgroundColor: '#4a4a4a',
                },
            },
        },
        MuiButton: {
            defaultProps: {
                disableElevation: true,
            },
            styleOverrides: {
                root: {
                    borderRadius: 999,
                    textTransform: 'none',
                },
            },
        },
        MuiPaper: {
            styleOverrides: {
                root: {
                    backgroundImage: 'none',
                },
            },
        },
        MuiDialog: {
            styleOverrides: {
                paper: {
                    borderRadius: 5,
                },
            },
        },
        MuiLinearProgress: {
            styleOverrides: {
                root: {
                    height: 6,
                    borderRadius: 3,
                },
                bar: {
                    borderRadius: 3,
                },
            },
        },
        MuiOutlinedInput: {
            styleOverrides: {
                root: {
                    borderRadius: 5,
                },
            },
        },
        MuiChip: {
            styleOverrides: {
                root: {
                    fontWeight: 600,
                },
            },
        },
    },
})

export default theme
