import { CheckCircle2, Clock3, UserRound, UsersRound, X } from 'lucide-react'
import { useId, useLayoutEffect, useRef } from 'react'
import { formats, languages, lessonLabel, money, slotLabel, timeLabel } from './lib'

export function Dialog({ title, children, onClose, busy, wide }) {
  const titleId = useId()
  const ref = useRef(null)
  const headingRef = useRef(null)
  useLayoutEffect(() => {
    const dialog = ref.current
    const trigger = document.activeElement
    dialog.showModal()
    return () => {
      dialog.close()
      if (trigger?.isConnected) trigger.focus({ preventScroll: true })
    }
  }, [])
  useLayoutEffect(() => {
    headingRef.current?.focus({ preventScroll: true })
  }, [title])

  return (
    <dialog ref={ref} className={wide ? 'dialog wide-dialog' : 'dialog'} aria-labelledby={titleId} onCancel={event => {
      event.preventDefault()
      if (!busy) onClose()
    }} onClick={event => {
      if (event.target === event.currentTarget && !busy) onClose()
    }}>
      <div className="dialog-content">
        <div className="dialog-heading">
          <h2 ref={headingRef} tabIndex={-1} id={titleId}>{title}</h2>
          <button type="button" className="icon-button" aria-label="Закрыть" onClick={onClose} disabled={busy}><X size={22} /></button>
        </div>
        {children}
      </div>
    </dialog>
  )
}

export function ErrorMessage({ children }) {
  return children ? <p role="alert" className="form-error">{children}</p> : null
}

export function AuthForm({ mode, onSwitch, onSubmit, busy, error, onBack, onRecovery, school, notice }) {
  const register = mode === 'register'
  return (
    <form className="dialog-form" onSubmit={event => {
      event.preventDefault()
      onSubmit(Object.fromEntries(new FormData(event.currentTarget)))
    }}>
      <p className="muted">{register ? 'В профиле будут храниться ваши записи и абонемент.' : 'Откройте своё расписание и управление абонементом.'}</p>
      {notice && <p className="success" role="status">{notice}</p>}
      {register && <label>Ваше имя<input name="name" autoComplete="name" placeholder="Например, Анна" minLength={2} maxLength={60} required /></label>}
      <label>Почта<input type="email" name="email" autoComplete="email" placeholder="you@example.com" maxLength={254} required /></label>
      <label>Пароль<input type="password" name="password" autoComplete={register ? 'new-password' : 'current-password'} minLength={register ? 8 : undefined} maxLength={256} placeholder={register ? 'Не менее 8 символов' : 'Ваш пароль'} required /></label>
      {register && school?.privacyUrl && <p className="muted demo-note">Как обрабатываются данные — <a className="text-button" href={school.privacyUrl} target="_blank" rel="noreferrer">политика конфиденциальности</a>.</p>}
      <ErrorMessage>{error}</ErrorMessage>
      <button className="button button-primary button-full" disabled={busy}>{busy ? 'Подождите…' : register ? 'Создать профиль' : 'Войти'}</button>
      <div className="auth-switch">{register ? 'Уже есть профиль?' : 'Впервые здесь?'} <button type="button" className="text-button" disabled={busy} onClick={onSwitch}>{register ? 'Войти' : 'Зарегистрироваться'}</button></div>
      <div className="auth-options">
        {onBack && <button type="button" className="text-button" onClick={onBack} disabled={busy}>Назад</button>}
        {!register && onRecovery && <button type="button" className="text-button" onClick={onRecovery} disabled={busy}>Забыли пароль?</button>}
        {!register && !onRecovery && school?.supportUrl && <a className="text-button" href={school.supportUrl}>Помощь со входом</a>}
      </div>
    </form>
  )
}

export function RecoveryForm({ reset, sent, busy, error, onSubmit, onBack }) {
  return <form className="dialog-form" onSubmit={event => {
    event.preventDefault()
    onSubmit(Object.fromEntries(new FormData(event.currentTarget)))
  }}>
    {sent ? <p role="status" className="info">Если профиль зарегистрирован на эту почту, на неё придёт ссылка для восстановления.</p>
      : reset
        ? <label>Новый пароль<input type="password" name="password" autoComplete="new-password" minLength={8} maxLength={256} required /></label>
        : <label>Почта аккаунта<input type="email" name="email" autoComplete="email" maxLength={254} required /></label>}
    <ErrorMessage>{error}</ErrorMessage>
    {!sent && <button className="button button-primary" disabled={busy}>{reset ? 'Установить пароль' : 'Получить ссылку'}</button>}
    <button type="button" className="text-button" disabled={busy} onClick={onBack}>Вернуться ко входу</button>
  </form>
}

export function ActivatePlan({ plan, busy, error, account, onSubmit, mode, school }) {
  const Icon = plan.format === 'group' ? UsersRound : UserRound
  const active = account?.subscription?.active
  const disabled = mode === 'disabled'
  return (
    <div className="dialog-form">
      <div className="plan-summary">
        <span className="option-icon"><Icon size={22} /></span>
        <div><strong>{plan.name}</strong><p>{lessonLabel(plan.lessons)} · {plan.days} дней</p></div>
      </div>
      {active && <p className="info">У вас уже есть действующий пакет.</p>}
      {!active && mode === 'demo' && <p className="info">Демо-пакет активируется бесплатно на {plan.days} дней.</p>}
      {!active && mode === 'live' && <p className="info">Разовая оплата {money(plan.price)}. Пакет действует {plan.days} дней со дня покупки.</p>}
      {disabled && <p className="info">Онлайн-оплата пока недоступна. {school?.supportUrl && <a className="text-button" href={school.supportUrl}>Связаться со школой</a>}</p>}
      {school?.termsUrl && <a className="text-button" href={school.termsUrl} target="_blank" rel="noreferrer">Условия оплаты и возврата</a>}
      <ErrorMessage>{error}</ErrorMessage>
      <button className="button button-primary button-full" onClick={onSubmit} disabled={active || disabled || busy}>
        {busy ? 'Подождите…' : !account ? 'Продолжить с профилем' : mode === 'live' ? 'Перейти к оплате' : 'Активировать демо-пакет'}
      </button>
    </div>
  )
}

export function Confirm({ text, busy, error, onSubmit }) {
  return (
    <div className="dialog-form">
      <p className="muted">{text}</p>
      <ErrorMessage>{error}</ErrorMessage>
      <button className="button button-danger button-full" disabled={busy} onClick={onSubmit}>{busy ? 'Сохраняем…' : 'Подтвердить отмену'}</button>
    </div>
  )
}

export function Booked({ booking, onCabinet }) {
  const slot = booking?.slot
  return (
    <div className="dialog-form">
      {slot && <div className="plan-summary">
        <span className="option-icon"><CheckCircle2 size={24} /></span>
        <div><strong>{languages[slot.language]} · {formats[slot.format]}</strong><p>{slotLabel(slot.startsAt)} · {timeLabel(slot.startsAt)} МСК</p></div>
      </div>}
      <p className="muted"><Clock3 size={16} /> Подробности занятия доступны в личном кабинете.</p>
      <button className="button button-primary button-full" onClick={onCabinet}>Мои занятия</button>
    </div>
  )
}
