import { lazy, Suspense, useEffect, useState } from 'react'
import { API, getToken, setToken } from './api'
import Login from './pages/Login'

// 懒加载各页：把 uPlot（总览）、xterm（终端）等大依赖拆成独立 chunk，
// 只在打开对应页时才下载，减小首屏 JS。
const Overview = lazy(() => import('./pages/Overview'))
const Workloads = lazy(() => import('./pages/Workloads'))
const Logs = lazy(() => import('./pages/Logs'))
const Terminal = lazy(() => import('./pages/Terminal'))
const Alerts = lazy(() => import('./pages/Alerts'))

type Tab = 'overview' | 'workloads' | 'logs' | 'terminal' | 'alerts'

const TABS: { key: Tab; label: string }[] = [
  { key: 'overview', label: '总览' },
  { key: 'workloads', label: '工作负载' },
  { key: 'logs', label: '日志' },
  { key: 'terminal', label: '终端' },
  { key: 'alerts', label: '告警' },
]

type Gate = 'checking' | 'login' | 'app'

export default function App() {
  const [tab, setTab] = useState<Tab>('overview')
  const [gate, setGate] = useState<Gate>('checking')
  const [loginEnabled, setLoginEnabled] = useState(true)

  useEffect(() => {
    (async () => {
      try {
        const c = await API.authConfig()
        setLoginEnabled(c.loginEnabled)
        if (!c.authEnabled) {
          setGate('app') // 后端未启用鉴权
          return
        }
        setGate(getToken() ? 'app' : 'login')
      } catch {
        setGate('app') // 拿不到配置先进，遇 401 再回登录
      }
    })()
  }, [])

  function toLogin() {
    setToken('')
    setGate('login')
  }

  if (gate === 'checking') {
    return <div className="login-bg"><div className="login-card">加载中…</div></div>
  }
  if (gate === 'login') {
    return <Login loginEnabled={loginEnabled} onSuccess={() => setGate('app')} />
  }

  return (
    <div className="app">
      <div className="sidebar">
        <h1>k8smanage<br /><span className="brand-sub">k3s 管理 + 监控</span></h1>
        <div className="nav">
          {TABS.map((t) => (
            <button key={t.key} className={tab === t.key ? 'active' : ''} onClick={() => setTab(t.key)}>
              {t.label}
            </button>
          ))}
        </div>
        <div className="foot">
          <a onClick={toLogin}>退出登录</a>
        </div>
      </div>

      <div className="main">
        <Suspense fallback={<div style={{ color: 'var(--muted)' }}>加载中…</div>}>
          {tab === 'overview' && <Overview onAuthError={toLogin} />}
          {tab === 'workloads' && <Workloads onAuthError={toLogin} />}
          {tab === 'logs' && <Logs onAuthError={toLogin} />}
          {tab === 'terminal' && <Terminal onAuthError={toLogin} />}
          {tab === 'alerts' && <Alerts onAuthError={toLogin} />}
        </Suspense>
      </div>
    </div>
  )
}
