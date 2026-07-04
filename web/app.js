// Terrain Sunset — Interactive Map Application
// Queries the Go backend API and renders terrain-corrected sun data.

const API = '/api';

// Default location: Innsbruck, Austria (deep Alpine valley — dramatic terrain effects)
const DEFAULT_LAT = 47.2692;
const DEFAULT_LON = 11.4041;

// --- State ---
let map;
let marker;
let currentLat = DEFAULT_LAT;
let currentLon = DEFAULT_LON;
let horizonProfile = null; // cached horizon elevations per azimuth

// --- Initialize ---
function init() {
  map = L.map('map').setView([DEFAULT_LAT, DEFAULT_LON], 11);

  // Dark tile layer (CartoDB dark)
  L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>',
    maxZoom: 19
  }).addTo(map);

  // Draggable marker
  marker = L.marker([DEFAULT_LAT, DEFAULT_LON], { draggable: true }).addTo(map);

  marker.on('dragend', () => {
    const pos = marker.getLatLng();
    currentLat = pos.lat;
    currentLon = pos.lng;
    updateDisplay();
  });

  map.on('click', (e) => {
    const pos = e.latlng;
    currentLat = pos.lat;
    currentLon = pos.lng;
    marker.setLatLng(pos);
    updateDisplay();
  });

  // Initial load
  updateDisplay();
}

// --- API Calls ---

async function fetchHorizon(lat, lon) {
  const resp = await fetch(`${API}/horizon?lat=${lat}&lon=${lon}`);
  if (!resp.ok) throw new Error(`horizon API: ${resp.status}`);
  return resp.json();
}

async function fetchTimes(lat, lon, date) {
  const ds = date || new Date().toISOString().slice(0, 10);
  const resp = await fetch(`${API}/times?lat=${lat}&lon=${lon}&date=${ds}`);
  if (!resp.ok) throw new Error(`times API: ${resp.status}`);
  return resp.json();
}

async function fetchSunPath(lat, lon, date) {
  const ds = date || new Date().toISOString().slice(0, 10);
  const resp = await fetch(`${API}/sunpath?lat=${lat}&lon=${lon}&date=${ds}`);
  if (!resp.ok) throw new Error(`sunpath API: ${resp.status}`);
  return resp.json();
}

async function fetchSunNow(lat, lon) {
  const resp = await fetch(`${API}/sunnow?lat=${lat}&lon=${lon}`);
  if (!resp.ok) throw new Error(`sunnow API: ${resp.status}`);
  return resp.json();
}

// --- Update Display ---

async function updateDisplay() {
  const status = document.getElementById('status');
  status.textContent = 'Computing horizon profile…';

  document.getElementById('lat-display').textContent = currentLat.toFixed(4);
  document.getElementById('lon-display').textContent = currentLon.toFixed(4);

  try {
    // Fetch horizon profile, times, sun path, and current sun position in parallel.
    const [profile, times, sunPath, sunNow] = await Promise.all([
      fetchHorizon(currentLat, currentLon),
      fetchTimes(currentLat, currentLon),
      fetchSunPath(currentLat, currentLon),
      fetchSunNow(currentLat, currentLon),
    ]);

    horizonProfile = profile;
    renderTimes(times);
    renderChart(profile, sunPath, sunNow);
    status.textContent = '';
  } catch (err) {
    console.error(err);
    status.textContent = '⚠️ ' + err.message;
  }
}

// --- Render Times ---

function renderTimes(times) {
  const elevEl = document.getElementById('elev-display');
  if (times.elevation === undefined || times.elevation === null) {
    elevEl.textContent = '—';
  } else {
    elevEl.textContent = Math.round(times.elevation);
  }

  // Helper: parse ISO8601 and format as local time
  function fmtLocal(iso) {
    if (!iso) return '—:—:—';
    const d = new Date(iso);
    if (isNaN(d.getTime())) return iso; // fallback
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  // Sunrise
  document.getElementById('sunrise-time').textContent = fmtLocal(times.correctedSunrise);
  document.getElementById('sunrise-standard').textContent =
    times.standardSunrise ? `Standard: ${fmtLocal(times.standardSunrise)}` : '';
  document.getElementById('sunrise-diff').textContent = formatDelay(times.sunriseDelayMinutes);
  setDiffClass('sunrise-diff', times.sunriseDelayMinutes);

  // Noon
  document.getElementById('noon-time').textContent = fmtLocal(times.solarNoon);

  // Sunset
  document.getElementById('sunset-time').textContent = fmtLocal(times.correctedSunset);
  document.getElementById('sunset-standard').textContent =
    times.standardSunset ? `Standard: ${fmtLocal(times.standardSunset)}` : '';
  document.getElementById('sunset-diff').textContent = formatDelay(times.sunsetDelayMinutes);
  setDiffClass('sunset-diff', times.sunsetDelayMinutes);
}

function formatDelay(minutes) {
  if (minutes === undefined || minutes === null) return '';
  if (minutes === 0) return '±0 min (same as flat horizon)';
  const sign = minutes > 0 ? '+' : '';
  const m = Math.abs(Math.round(minutes));
  const h = Math.floor(m / 60);
  const rem = m % 60;
  let str = sign + m + ' min';
  if (h > 0) str += ` (${h}h ${rem}m)`;
  str += minutes > 0 ? ' later' : ' earlier';
  return str;
}

function setDiffClass(id, minutes) {
  const el = document.getElementById(id);
  el.className = 'time-diff';
  if (minutes === undefined || minutes === null || minutes === 0) {
    el.classList.add('zero');
  } else if (Math.abs(minutes) < 1) {
    el.classList.add('zero');
  } else if (minutes > 0) {
    el.classList.add('positive');
  } else {
    el.classList.add('negative');
  }
}

// --- Render Polar Chart ---

function renderChart(profile, sunPath, sunNow) {
  const canvas = document.getElementById('horizon-chart');
  const ctx = canvas.getContext('2d');
  const w = canvas.width;
  const h = canvas.height;
  const cx = w / 2;
  const cy = h / 2;
  const maxR = w / 2 - 30;

  ctx.clearRect(0, 0, w, h);

  // Background circles
  for (let r = 30; r <= 90; r += 30) {
    const radius = (r / 90) * maxR;
    ctx.beginPath();
    ctx.arc(cx, cy, radius, 0, 2 * Math.PI);
    ctx.strokeStyle = '#1e3a5f';
    ctx.lineWidth = 0.5;
    ctx.stroke();

    // Label
    ctx.fillStyle = '#556677';
    ctx.font = '9px sans-serif';
    ctx.fillText(r + '°', cx + 3, cy - radius + 10);
  }

  // Horizon profile
  if (profile && profile.elevations) {
    ctx.beginPath();
    const steps = profile.elevations.length;
    for (let i = 0; i <= steps; i++) {
      const idx = i % steps;
      const az = idx / steps * 360;
      const elev = profile.elevations[idx];
      const r = Math.max(0, Math.min(maxR, ((elev + 10) / 100) * maxR));
      const rad = azToRad(az);
      const x = cx + r * Math.sin(rad);
      const y = cy - r * Math.cos(rad);
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    }
    ctx.closePath();
    ctx.fillStyle = 'rgba(255, 87, 34, 0.55)';
    ctx.fill();
    ctx.strokeStyle = '#ff5722';
    ctx.lineWidth = 4;
    ctx.stroke();
  }

  // Sun path
  if (sunPath && sunPath.points) {
    const points = sunPath.points;

    // Build continuous sub-paths for visible (green) and non-visible (yellow) segments.
    const visiblePaths = [];
    const hiddenPaths = [];
    let current = null;
    let currentVisible = null;

    for (let i = 0; i < points.length; i++) {
      const p = points[i];
      const pt = polarToCanvas(p.azimuth, p.elevation, cx, cy, maxR);

      if (current === null) {
        current = [pt];
        currentVisible = p.visible;
      } else if (p.visible === currentVisible) {
        current.push(pt);
      } else {
        // Visibility changed — store current path and start new one.
        if (currentVisible) visiblePaths.push(current);
        else hiddenPaths.push(current);
        current = [current[current.length - 1], pt];
        currentVisible = p.visible;
      }
    }
    if (current !== null) {
      if (currentVisible) visiblePaths.push(current);
      else hiddenPaths.push(current);
    }

    // Draw hidden (below-horizon) portions in yellow.
    ctx.strokeStyle = '#ffb300';
    ctx.lineWidth = 2;
    for (const path of hiddenPaths) drawPath(ctx, path);

    // Draw visible (above-horizon) portions in green.
    ctx.strokeStyle = '#66bb6a';
    ctx.lineWidth = 2;
    for (const path of visiblePaths) drawPath(ctx, path);

    // Sun position dot at current time.
    if (sunNow && sunNow.azimuth !== undefined && sunNow.elevation !== undefined) {
      const pt = polarToCanvas(sunNow.azimuth, sunNow.elevation, cx, cy, maxR);
      ctx.beginPath();
      ctx.arc(pt.x, pt.y, 6, 0, 2 * Math.PI);
      ctx.fillStyle = '#ff9800';
      ctx.fill();
      ctx.strokeStyle = '#fff';
      ctx.lineWidth = 2;
      ctx.stroke();
    }
  }

  // Cardinal direction labels
  const dirs = [
    { label: 'N', az: 0 },
    { label: 'E', az: 90 },
    { label: 'S', az: 180 },
    { label: 'W', az: 270 },
  ];
  ctx.fillStyle = '#8899aa';
  ctx.font = 'bold 11px sans-serif';
  for (const d of dirs) {
    const rad = azToRad(d.az);
    const lx = cx + (maxR + 16) * Math.sin(rad);
    const ly = cy - (maxR + 16) * Math.cos(rad);
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(d.label, lx, ly);
  }
}

function azToRad(az) {
  // az = 0° = North, 90° = East (clockwise).
  // Canvas: 0 radians = right (East), π/2 = down (South).
  // Conversion: canvas_angle = π/2 - az_rad = π/2 - (az * π/180)
  return (Math.PI / 180) * (90 - az);
}

function polarToCanvas(az, elev, cx, cy, maxR) {
  const r = Math.max(0, Math.min(maxR, ((elev + 10) / 100) * maxR));
  const rad = azToRad(az);
  return {
    x: cx + r * Math.sin(rad),
    y: cy - r * Math.cos(rad),
  };
}

function drawPath(ctx, points) {
  if (points.length < 2) return;
  ctx.beginPath();
  ctx.moveTo(points[0].x, points[0].y);
  for (let i = 1; i < points.length; i++) {
    ctx.lineTo(points[i].x, points[i].y);
  }
  ctx.stroke();
}

// --- Boot ---
document.addEventListener('DOMContentLoaded', init);
