// API 客户端：注入鉴权头 / WS token；401 抛 Unauthorized 供上层提示填 token。

const TOKEN_KEY = 'k8sm_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) || ''
}
export function setToken(t: string) {
  localStorage.setItem(TOKEN_KEY, t)
}

export class Unauthorized extends Error {}

// 子路径前缀：由后端注入 index.html 的 window.__BASE_PATH__（如 /k8smanage），空=根。
const BASE: string = (typeof window !== 'undefined' && (window as any).__BASE_PATH__) || ''

async function req(path: string, opts: RequestInit = {}): Promise<any> {
  const headers: Record<string, string> = { ...(opts.headers as any) }
  const t = getToken()
  if (t) headers['Authorization'] = 'Bearer ' + t
  const res = await fetch(BASE + '/api' + path, { ...opts, headers })
  if (res.status === 401) throw new Unauthorized('需要访问令牌')
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = (await res.json()).error || msg
    } catch {}
    throw new Error(msg)
  }
  const ct = res.headers.get('content-type') || ''
  return ct.includes('json') ? res.json() : null
}

export interface NodeOverview {
  name: string; ready: boolean; roles: string[]; kubeletVersion: string; cpuCores: number
  cpu: number; memUse: number; memTot: number; diskUse: number; diskTot: number; load1: number; hasData: boolean
}
export interface Overview {
  nodes: NodeOverview[]
  workloads: { total: number; ready: number }
}
export interface Workload {
  namespace: string; kind: string; name: string; desired: number; ready: number
  pausable: boolean; paused: boolean; image: string; createdAt: string
}
export interface Pod { namespace: string; name: string; phase: string; node: string; ready: boolean; containers: string[] }
export interface Point { TS: string; Value: number }
export interface Rule {
  id: string; name: string; kind: string; target: string; metric: string
  cmp: string; threshold: number; forSec: number; enabled: boolean
}
export interface ActiveAlert {
  ruleId: string; ruleName: string; kind: string; target: string; metric: string
  state: string; value: number; threshold: number; since: string
}

export const API = {
  authConfig: (): Promise<{ authEnabled: boolean; loginEnabled: boolean }> => req('/config'),
  login: (username: string, password: string): Promise<{ token: string }> =>
    req('/login', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ username, password }) }),
  overview: (): Promise<Overview> => req('/overview'),
  workloads: (ns = ''): Promise<Workload[]> => req('/workloads' + (ns ? `?ns=${encodeURIComponent(ns)}` : '')),
  pods: (ns = ''): Promise<Pod[]> => req('/pods' + (ns ? `?ns=${encodeURIComponent(ns)}` : '')),
  query: (kind: string, target: string, metric: string, from?: number, to?: number): Promise<Point[]> =>
    req(`/metrics/query?kind=${kind}&target=${encodeURIComponent(target)}&metric=${metric}` +
      (from ? `&from=${from}` : '') + (to ? `&to=${to}` : '')),
  scale: (ns: string, kind: string, name: string, replicas: number) =>
    req(`/workloads/${ns}/${kind}/${name}/scale`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ replicas }),
    }),
  restart: (ns: string, kind: string, name: string) => req(`/workloads/${ns}/${kind}/${name}/restart`, { method: 'POST' }),
  pause: (ns: string, kind: string, name: string) => req(`/workloads/${ns}/${kind}/${name}/pause`, { method: 'POST' }),
  resume: (ns: string, kind: string, name: string) => req(`/workloads/${ns}/${kind}/${name}/resume`, { method: 'POST' }),
  delPod: (ns: string, name: string) => req(`/pods/${ns}/${name}`, { method: 'DELETE' }),
  rules: (): Promise<Rule[]> => req('/alerts/rules'),
  saveRule: (r: Rule): Promise<Rule> =>
    req('/alerts/rules', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(r) }),
  delRule: (id: string) => req(`/alerts/rules/${id}`, { method: 'DELETE' }),
  active: (): Promise<ActiveAlert[]> => req('/alerts/active'),
}

// wsURL 拼装 WebSocket 地址（带 token 查询参数；含子路径前缀）。
export function wsURL(path: string): string {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const sep = path.includes('?') ? '&' : '?'
  return `${proto}://${location.host}${BASE}/api${path}${sep}token=${encodeURIComponent(getToken())}`
}
