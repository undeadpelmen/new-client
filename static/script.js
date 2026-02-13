async function fetchData() {
    try {
        const response = await fetch('/api/v1/state');
        const data = await response.json();

        if (data.status === 'success') {
            const d = data.data;
            document.getElementById('temp').textContent = d.sensors.temperature.toFixed(1);
            document.getElementById('humidity').textContent = d.sensors.humidity.toFixed(1);

            document.getElementById('light-status').textContent = d.relays.light ? 'Вкл' : 'Выкл';
            document.getElementById('light-status').className = d.relays.light ? 'status-on' : 'status-off';

            document.getElementById('heater-status').textContent = d.relays.heater ? 'Вкл' : 'Выкл';
            document.getElementById('heater-status').className = d.relays.heater ? 'status-on' : 'status-off';

            document.getElementById('pump-status').textContent = d.relays.pump ? 'Вкл' : 'Выкл';
            document.getElementById('pump-status').className = d.relays.pump ? 'status-on' : 'status-off';

            document.getElementById('system-mode').textContent = d.system.mode;
            document.getElementById('cycle-count').textContent = d.system.cycle_count;
            document.getElementById('uptime').textContent = d.system.uptime;

            document.getElementById('sensor-status').textContent = d.sensors.sensor_error ?
                'Ошибка датчика!' : 'Датчик OK';
            document.getElementById('sensor-status').className = d.sensors.sensor_error ? 'error' : '';
        }
    } catch (error) {
        console.error('Ошибка:', error);
    }
}

async function toggleMock() {
    const response = await fetch('/api/v1/mock', { method: 'POST' });
    const data = await response.json();
    alert(data.message);
    fetchData();
}

async function testSensor() {
    const response = await fetch('/api/v1/sensor/test');
    const data = await response.json();
    if (data.status === 'success') {
        alert(`Температура: ${data.data.temperature}°C\nВлажность: ${data.data.humidity}%`);
    } else {
        alert('Ошибка датчика: ' + data.message);
    }
}

setInterval(fetchData, 10000);