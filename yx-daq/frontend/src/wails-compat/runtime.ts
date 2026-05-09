import { Events } from '@wailsio/runtime'

export const EventsOn = (eventName: string, callback: (data: any) => void) => {
  return Events.On(eventName, event => callback(event.data))
}

export const EventsOff = (eventName: string, ...additionalEventNames: string[]) => {
  Events.Off(eventName, ...additionalEventNames)
}

export const EventsOffAll = () => Events.OffAll()
export const EventsOnce = (eventName: string, callback: (data: any) => void) => {
  return Events.Once(eventName, event => callback(event.data))
}
export const EventsEmit = (eventName: string, ...args: any[]) => Events.Emit(eventName, args.length <= 1 ? args[0] : args)
