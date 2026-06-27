import { ReactNode } from 'react'

// Modal 通用弹窗：标题 + 内容（含按钮）；点遮罩或调用 onClose 关闭。
export default function Modal({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <div className="modal-bg" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>{title}</h2>
        {children}
      </div>
    </div>
  )
}

// ModalActions 弹窗底部按钮区（右对齐）。
export function ModalActions({ children }: { children: ReactNode }) {
  return <div className="modal-actions">{children}</div>
}
