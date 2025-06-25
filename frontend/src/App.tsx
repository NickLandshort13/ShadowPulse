import { useMap } from 'react-leaflet';
import React from 'react';
import { MapContainer, TileLayer } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';
import 'leaflet.markercluster/dist/MarkerCluster.css';
import 'leaflet.markercluster/dist/MarkerCluster.Default.css';
import * as L from 'leaflet';
import 'leaflet.markercluster';
import './App.css'

import iconUrl from 'leaflet/dist/images/marker-icon.png?url';
import shadowUrl from 'leaflet/dist/images/marker-shadow.png?url';

L.Icon.Default.mergeOptions({
  iconUrl,
  shadowUrl,
  iconSize: [25, 41],
  iconAnchor: [12, 41],
  popupAnchor: [1, -34],
  shadowSize: [41, 41],
});


interface ProxyData {
  ip: string;
  port: number;
  latency: number;
  country: string;
  city: string;
  lat: number;
  lng: number;
  type: string;
  lastCheck: string;
}

const MarkerCluster: React.FC = () => {
  const map = useMap();
  const [proxies, setProxies] = React.useState<ProxyData[]>([]);

  React.useEffect(() => {
    if (!map) return;

    const fetchProxies = async () => {
      try {
        const res = await fetch('http://localhost:8080/api/proxies');
        if (!res.ok) throw new Error('Network response was not ok');
        const data = await res.json();
        setProxies(data);
      } catch (error) {
        console.error('Error fetching proxies:', error);
      }
    };

    fetchProxies();
  }, [map]);

  React.useEffect(() => {
    if (!map || proxies.length === 0) return;

    const markers = L.markerClusterGroup();

    proxies.forEach(proxy => {
      const marker = L.marker([proxy.lat, proxy.lng]).bindPopup(`
        <strong>${proxy.ip}:${proxy.port}</strong><br/>
        Type: ${proxy.type}<br/>
        Country: ${proxy.country}<br/>
        City: ${proxy.city}<br/>
        Latency: ${proxy.latency}ms
      `);

      const color = proxy.latency < 100 ? '#4CAF50' : proxy.latency < 300 ? '#FFC107' : '#F44336';
      const html = `
        <div style="
          background: ${color};
          width: 24px;
          height: 24px;
          border-radius: 20%;
          display: flex;
          align-items: center;
          justify-content: center;
          color: black;
          font-weight: bold;
          font-size: 12px;
          border: 2px solid white;
        ">${proxy.latency}</div>
      `;

      const icon = L.divIcon({ html, className: 'custom-cluster-icon', iconSize: [24, 24] });
      marker.setIcon(icon);

      markers.addLayer(marker);
    });

    map.addLayer(markers);

    return () => {
      map.removeLayer(markers);
    };
  }, [map, proxies]);

  return null;
};

const App: React.FC = () => {
  return (
    <div style={{ height: '100vh', width: '100vw' }}>
      <MapContainer center={[37.8, -96.9]} zoom={5} style={{ height: '100%', width: '100%' }}>
        <TileLayer
          url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png" 
          attribution=""
        />
        <MarkerCluster />
      </MapContainer>
    </div>
  );
};

export default App;