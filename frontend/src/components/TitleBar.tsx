import {useEffect, useState, type CSSProperties} from 'react'
import {useTranslation} from 'react-i18next'
import AppBar from '@mui/material/AppBar'
import Box from '@mui/material/Box'
import IconButton from '@mui/material/IconButton'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import CloseIcon from '@mui/icons-material/Close'
import FilterNoneIcon from '@mui/icons-material/FilterNone'
import HorizontalRuleIcon from '@mui/icons-material/HorizontalRule'
import CropSquareIcon from '@mui/icons-material/CropSquare'
import {Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise} from '../../wailsjs/runtime/runtime'
import {TITLE_BAR_HEIGHT} from '../theme'
import appIcon from '../assets/app-icon.png'

const dragRegion: CSSProperties = {'--wails-draggable': 'drag'} as CSSProperties
const noDragRegion: CSSProperties = {'--wails-draggable': 'no-drag'} as CSSProperties

export default function TitleBar() {
    const {t} = useTranslation()
    const [maximised, setMaximised] = useState(false)

    // Resyncs from the real window state on every resize, so the icon
    // stays correct even when maximise/restore happens via Windows Snap,
    // a keyboard shortcut, or dragging to the screen edge — not just our
    // own button/double-click.
    useEffect(() => {
        const syncMaximised = () => {
            WindowIsMaximised().then(setMaximised).catch(() => {})
        }
        syncMaximised()
        window.addEventListener('resize', syncMaximised)
        return () => window.removeEventListener('resize', syncMaximised)
    }, [])

    const toggleMaximise = () => {
        WindowToggleMaximise()
        setMaximised((m) => !m)
    }

    return (
        <AppBar
            position="static"
            color="transparent"
            elevation={0}
            sx={{
                height: TITLE_BAR_HEIGHT,
                borderBottom: 1,
                borderColor: 'divider',
                flex: '0 0 auto',
            }}
        >
            <Toolbar
                variant="dense"
                disableGutters
                onDoubleClick={toggleMaximise}
                style={dragRegion}
                sx={{height: TITLE_BAR_HEIGHT, minHeight: `${TITLE_BAR_HEIGHT}px !important`, pl: 1.5}}
            >
                <Box component="img" src={appIcon} alt="" sx={{width: 16, height: 16, mr: 1}} style={noDragRegion}/>
                <Typography variant="body2" sx={{fontWeight: 600, flex: 1}}>
                    {t('app.title')}
                </Typography>

                <Box sx={{display: 'flex', height: '100%'}} style={noDragRegion}>
                    <IconButton
                        aria-label={t('titleBar.minimize') ?? 'Minimize'}
                        onClick={() => WindowMinimise()}
                        sx={{borderRadius: 0, width: 46, height: '100%'}}
                    >
                        <HorizontalRuleIcon fontSize="small"/>
                    </IconButton>
                    <IconButton
                        aria-label={t('titleBar.maximize') ?? 'Maximize'}
                        onClick={toggleMaximise}
                        sx={{borderRadius: 0, width: 46, height: '100%'}}
                    >
                        {maximised ? <FilterNoneIcon sx={{fontSize: 14}}/> : <CropSquareIcon sx={{fontSize: 14}}/>}
                    </IconButton>
                    <IconButton
                        aria-label={t('titleBar.close') ?? 'Close'}
                        onClick={() => Quit()}
                        sx={{
                            borderRadius: 0,
                            width: 46,
                            height: '100%',
                            '&:hover': {backgroundColor: 'error.main', color: '#fff'},
                        }}
                    >
                        <CloseIcon fontSize="small"/>
                    </IconButton>
                </Box>
            </Toolbar>
        </AppBar>
    )
}
