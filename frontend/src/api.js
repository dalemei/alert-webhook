import axios from 'axios';

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
});

export async function getStats() {
  const { data } = await api.get('/stats');
  return data;
}

export async function getAlerts(params) {
  const { data } = await api.get('/alerts', { params });
  return data;
}

export async function getAlertDetail(id) {
  const { data } = await api.get(`/alerts/${id}`);
  return data;
}
