import 'leaflet';

declare module 'leaflet' {
  export const markerClusterGroup: (options?: any) => any;
}