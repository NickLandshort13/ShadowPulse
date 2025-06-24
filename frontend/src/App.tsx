import React, { useEffect, useState } from 'react';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';
import './leafletFix';
import { Box, Typography } from '@mui/material';
import L from 'leaflet';

interface Proxy {
  ip: string;
  port: number;
  latency: number;
  country: string;
  lat: number;
  lng: number;
}

const createCustomIcon = (latency: number) => {
  const color = latency < 100 ? 'green' : latency < 300 ? 'orange' : 'red';
  
  return L.divIcon({
    className: 'custom-marker',
    html: `
      <div style="
        background: ${color};
        width: 24px;
        height: 24px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        color: white;
        font-weight: bold;
        font-size: 12px;
        border: 2px solid white;
      ">
        ${latency}
      </div>
    `,
    iconSize: [24, 24],
    iconAnchor: [12, 12]
  });
};

const ShadowPulse: React.FC = () => {
  const [proxies] = useState<Proxy[]>([
    {
      ip: '192.168.1.1',
      port: 8080,
      latency: 120,
      country: 'US',
      lat: 37.0902,
      lng: -95.7129
    },
    {
      ip: '193.32.1.5',
      port: 3128,
      latency: 80,
      country: 'DE',
      lat: 51.1657,
      lng: 10.4515
    }
  ]);

  return (
    <Box sx={{ p: 2, height: '100vh' }}>
      <Typography variant="h4" gutterBottom>
        ShadowPulse
      </Typography>
      
      <div style={{ height: 'calc(100vh - 100px)', width: '100%' }}>
        <MapContainer 
          center={[20, 0]} 
          zoom={2} 
          style={{ height: '100%', width: '100%' }}
        >
          <TileLayer
            url="https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png"
            attribution=''
          />
          
          {proxies.map((proxy) => (
            <Marker 
              key={`${proxy.ip}:${proxy.port}`}
              position={[proxy.lat, proxy.lng]}
              icon={createCustomIcon(proxy.latency)}
            >
              <Popup>
                <div style={{ minWidth: '200px' }}>
                  <strong>{proxy.ip}:{proxy.port}</strong><br/>
                  Country: {proxy.country}<br/>
                  Latency: {proxy.latency}ms
                </div>
              </Popup>
            </Marker>
          ))}
        </MapContainer>
      </div>
    </Box>
  );
};

export default ShadowPulse;