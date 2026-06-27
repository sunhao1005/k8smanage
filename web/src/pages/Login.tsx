import { useState } from 'react'
import { API, setToken } from '../api'

// 登录页：就一个账号 + 密码。（后端若只配了 API Token 而无账密，则退化为填 Token。）
export default function Login({ loginEnabled, onSuccess }: { loginEnabled: boolean; onSuccess: () => void }) {
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [err, setErr] = useState('')

  async function doLogin() {
    setErr('')
    try {
      const { token } = await API.login(user, pass)
      setToken(token)
      onSuccess()
    } catch (e: any) {
      setErr(e.message || '登录失败')
    }
  }
  function applyToken() {
    setToken(pass.trim()) // 无账密时，把密码框当 Token 用
    onSuccess()
  }

  return (
    <div className="login-bg">
      <div className="login-card">
        <div className="login-brand">k8smanage</div>
        <div className="login-sub">k3s 管理 + 监控控制台</div>
        {err && <div className="err">{err}</div>}

        {loginEnabled ? (
          <>
            <input placeholder="用户名" value={user} onChange={(e) => setUser(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && doLogin()} />
            <input placeholder="密码" type="password" value={pass} onChange={(e) => setPass(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && doLogin()} />
            <button className="btn primary login-btn" onClick={doLogin}>登录</button>
          </>
        ) : (
          <>
            <input placeholder="API Token" type="password" value={pass} onChange={(e) => setPass(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && applyToken()} />
            <button className="btn primary login-btn" onClick={applyToken}>进入</button>
          </>
        )}
      </div>
    </div>
  )
}
