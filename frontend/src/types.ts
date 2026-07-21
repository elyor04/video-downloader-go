import {job as jobNS, utils as utilsNS} from '../wailsjs/go/models'

export type JobDTO = jobNS.DTO
export type ResolutionOption = utilsNS.ResolutionOption

export interface PreviewState {
    url: string
    state: 'idle' | 'fetching' | 'ready' | 'error'
    title: string
    thumbnail: string
    isPlaylist: boolean
    playlistCount: number
    error: string
}

export interface OptionsState {
    mode: string
    resolution: number
    convertTo: string
    convertOptions: string[]
}

export interface PlaylistPrompt {
    jobId: string
    count: number
}

export interface AuthPrompt {
    jobId: string
    url: string
}
