import { createRoot } from 'react-dom/client'
import { useEffect, useState } from 'react'
import '@fontsource-variable/manrope'
import { api } from './lib'
import { Cabinet } from './Cabinet'
import { BookingCalendar } from './Calendar'
import { ActivatePlan, AuthForm, Booked, Confirm, Dialog, ErrorMessage, RecoveryForm } from './Dialogs'
import { Footer, Header, Home } from './Home'
import { ModuleDetails } from './Learning'
import './styles.css'
import './studio.css'

const isCabinet = () => window.location.pathname === '/cabinet'
const paymentReturn = () => new URLSearchParams(window.location.search).get('order')
const storedSelection = 'lingua-checkout-selection'

function App() {
  const [cabinet, setCabinet] = useState(isCabinet)
  const [account, setAccount] = useState(null)
  const [accountState, setAccountState] = useState('loading')
  const [catalog, setCatalog] = useState(null)
  const [catalogError, setCatalogError] = useState('')
  const [modal, setModal] = useState(null)
  const [selection, setSelection] = useState({ format: 'trial', language: 'english', slotId: '', moduleId: '' })
  const [tab, setTab] = useState(() => paymentReturn() ? 'subscription' : 'lessons')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  async function reloadCatalog(signal) {
    setCatalogError('')
    try {
      setCatalog(await api('/catalog', { signal }))
    } catch (error) {
      if (error.name !== 'AbortError') setCatalogError(error.message)
    }
  }

  async function loadAccount(signal) {
    setAccountState('loading')
    try {
      const data = await api('/account', { signal })
      setAccount(data.account)
      if (data.account?.profile.teacher && !paymentReturn()) setTab('teaching')
      if (data.account && paymentReturn()) await checkOrder(paymentReturn(), signal)
      setAccountState('ready')
    } catch (error) {
      if (error.name !== 'AbortError') {
        setError(error.message)
        setAccountState('error')
      }
    }
  }

  useEffect(() => {
    const controller = new AbortController()
    reloadCatalog(controller.signal)
    loadAccount(controller.signal)
    if (window.location.hash.startsWith('#reset=')) {
      open({ kind: 'reset', token: window.location.hash.slice(7) })
      window.history.replaceState({}, '', '/cabinet')
    } else if (window.location.hash) {
      document.getElementById(window.location.hash.slice(1))?.scrollIntoView()
    }
    const onPop = () => { setCabinet(isCabinet()); setModal(null) }
    window.addEventListener('popstate', onPop)
    return () => {
      controller.abort()
      window.removeEventListener('popstate', onPop)
    }
  }, [])

  function open(next) {
    setError('')
    setModal(next)
  }

  function showCabinet(nextTab = account?.profile.teacher ? 'teaching' : 'lessons') {
    open(null)
    if (!isCabinet()) window.history.pushState({}, '', '/cabinet')
    setTab(nextTab)
    setCabinet(true)
    window.scrollTo(0, 0)
  }

  function showHome() {
    open(null)
    if (isCabinet()) window.history.pushState({}, '', '/')
    setCabinet(false)
    window.scrollTo(0, 0)
  }

  function openAccount() {
    if (account) showCabinet()
    else open({ kind: 'auth', mode: 'login', next: 'cabinet' })
  }

  function onBook(format = 'trial', language = account?.profile.language || 'english', moduleId = '') {
    setSelection({ format, language, moduleId, slotId: '' })
    open({ kind: 'booking' })
    reloadCatalog()
  }

  function onMove(booking) {
    const { format, language } = booking.slot
    setSelection({ format, language, moduleId: booking.moduleId || booking.slot.moduleId || '', slotId: '' })
    open({ kind: 'booking', reschedule: booking })
    reloadCatalog()
  }

  function onModule(item) {
    open({ kind: 'module', item })
  }

  function onPlan(plan, resume = false) {
    open({ kind: 'plan', plan, resume })
  }

  async function request(work) {
    if (busy) return
    setBusy(true)
    setError('')
    try {
      await work()
    } catch (error) {
      setError(error.message)
      if (error.status === 401 && modal?.kind !== 'auth') {
        setAccount(null)
        open({ kind: 'auth', mode: 'login', next: modal || 'cabinet' })
      }
    } finally {
      setBusy(false)
    }
  }

  function authenticate(form) {
    request(async () => {
      const data = await api('/' + modal.mode, { method: 'POST', body: form })
      setAccount(data.account)
      setAccountState('ready')
      reloadCatalog()
      if (paymentReturn()) {
        showCabinet('subscription')
        await checkOrder(paymentReturn())
      } else if (modal.next === 'cabinet') {
        showCabinet(data.account.profile.teacher ? 'teaching' : 'lessons')
      } else open(modal.next)
    })
  }

  function sendRecovery(form) {
    request(async () => {
      await api('/recovery', { method: 'POST', body: form })
      open({ ...modal, sent: true })
    })
  }

  function resetPassword(form) {
    request(async () => {
      await api('/reset-password', { method: 'POST', body: { ...form, token: modal.token } })
      setAccount(null)
      reloadCatalog()
      open({ kind: 'auth', mode: 'login', next: 'cabinet', notice: 'Пароль обновлён. Войдите с новым паролем.' })
    })
  }

  function continueBooking(plan) {
    if (!account) {
      open({ kind: 'auth', mode: 'register', next: modal })
      return
    }
    if (plan) {
      onPlan(plan, true)
      return
    }
    request(async () => {
      try {
        const move = modal.reschedule
        const data = await api('/bookings' + (move ? '/' + move.id : ''), {
          method: move ? 'PATCH' : 'POST', body: { slotId: selection.slotId, moduleId: selection.moduleId },
        })
        setAccount(data.account)
        setNotice('')
        open({ kind: 'booked', booking: data.account.bookings.find(item => item.slot.id === selection.slotId && !item.cancelled), moved: Boolean(move) })
      } finally {
        reloadCatalog()
      }
    })
  }

  async function checkOrder(id, signal) {
    const data = await api('/orders/' + encodeURIComponent(id) + '/refresh', { method: 'POST', signal })
    setAccount(data.account)
    if (paymentReturn()) window.history.replaceState({}, '', '/cabinet')
    if (data.order.status === 'paid') {
      setNotice('Оплата подтверждена. Пакет активирован.')
      const saved = sessionStorage.getItem(storedSelection)
      sessionStorage.removeItem(storedSelection)
      if (saved) {
        setSelection(JSON.parse(saved))
        open({ kind: 'booking' })
        reloadCatalog()
      }
    } else if (data.order.status === 'canceled') {
      sessionStorage.removeItem(storedSelection)
      setNotice('Платёж отменён. Вы можете оформить пакет повторно.')
    } else setNotice('Оплата пока ожидает подтверждения.')
  }

  async function beginPayment(plan) {
    const data = await api('/orders', { method: 'POST', body: { planId: plan.id } })
    if (modal.resume) sessionStorage.setItem(storedSelection, JSON.stringify(selection))
    if (data.order.url && data.order.status === 'pending') {
      window.location.assign(data.order.url)
      return
    }
    showCabinet('subscription')
    await checkOrder(data.order.id)
  }

  function activatePlan() {
    if (!account) {
      open({ kind: 'auth', mode: 'register', next: modal })
      return
    }
    request(async () => {
      if (catalog?.payments === 'live') {
        await beginPayment(modal.plan)
        return
      }
      const data = await api('/subscription', { method: 'POST', body: { planId: modal.plan.id } })
      setAccount(data.account)
      if (modal.resume) open({ kind: 'booking' })
      else {
        setNotice('Пакет активирован.')
        showCabinet('subscription')
      }
    })
  }

  function confirmCancellation() {
    request(async () => {
      const isBooking = modal.kind === 'cancel-booking'
      const path = isBooking ? '/bookings/' + modal.booking.id : '/subscription'
      const data = await api(path, { method: 'DELETE' })
      setAccount(data.account)
      setNotice(isBooking ? 'Запись отменена.' : 'Пакет отменён.')
      open(null)
      if (isBooking) reloadCatalog()
    })
  }

  function onPrepare(item, prepared) {
    request(async () => {
      const data = await api('/progress/' + encodeURIComponent(item.id), { method: 'PATCH', body: { prepared } })
      setAccount(data.account)
    })
  }

  async function saveProfile(form) {
    setBusy(true)
    setNotice('')
    try {
      const data = await api('/profile', { method: 'PATCH', body: form })
      setAccount(data.account)
    } finally {
      setBusy(false)
    }
  }

  function logout() {
    request(async () => {
      await api('/logout', { method: 'POST' })
      setAccount(null)
      setNotice('')
      sessionStorage.removeItem(storedSelection)
      reloadCatalog()
      showHome()
    })
  }

  const titles = {
    auth: modal?.mode === 'register' ? 'Создайте профиль' : 'Вход в кабинет',
    booking: modal?.reschedule ? 'Перенос занятия' : 'Календарь занятий',
    module: modal?.item?.title,
    recovery: 'Восстановление доступа',
    reset: 'Новый пароль',
    plan: 'Пакет занятий',
    booked: modal?.moved ? 'Занятие перенесено!' : 'Вы записаны!',
    'cancel-booking': 'Отменить запись?',
    'cancel-plan': 'Отменить демо-пакет?',
  }
  const school = catalog?.school
  const displayedModule = school?.modules.find(item => item.id === modal?.item?.id) || modal?.item

  return (
    <>
      <a className="skip-link" href="#main">К содержанию</a>
      <Header account={account} cabinet={cabinet} onAccount={openAccount} onBook={onBook} />
      {cabinet ? account
        ? <Cabinet account={account} catalog={catalog} catalogError={catalogError} tab={tab} setTab={next => { setNotice(''); setTab(next) }}
          onBook={onBook} onPlan={onPlan} onCancelBooking={booking => open({ kind: 'cancel-booking', booking })}
          onCancelPlan={() => open({ kind: 'cancel-plan' })} onSave={saveProfile} onLogout={logout} busy={busy} notice={notice}
          onReload={() => reloadCatalog()} onMove={onMove} onModule={onModule} onRefreshOrder={id => request(() => checkOrder(id))} />
        : <main id="main" className="wrap cabinet cabinet-guest">
          <h1>Личный кабинет</h1>
          {notice && <p className="success" role="status">{notice}</p>}
          <p className="muted">{accountState === 'loading'
            ? 'Загружаем профиль…'
            : accountState === 'error' ? 'Не удалось проверить состояние входа.' : 'Войдите, чтобы открыть свои занятия и прогресс.'}</p>
          {accountState === 'error' && <button className="button button-outline" onClick={() => loadAccount()}>Обновить</button>}
          {accountState === 'ready' && <button className="button button-primary" onClick={openAccount}>Войти</button>}
        </main>
        : <Home catalog={catalog} catalogError={catalogError} onReload={() => reloadCatalog()} onBook={onBook} onPlan={onPlan} onModule={onModule} />}
      {!modal && error && <div className="wrap"><ErrorMessage>{error}</ErrorMessage></div>}
      <Footer cabinet={cabinet} school={school} />
      {modal && <Dialog title={titles[modal.kind]} onClose={() => open(null)} busy={busy} wide={modal.kind === 'booking'}>
        {modal.kind === 'auth' && <AuthForm key={modal.mode} mode={modal.mode} busy={busy} error={error} onSubmit={authenticate} school={school} notice={modal.notice}
          onBack={modal.next !== 'cabinet' ? () => open(modal.next) : undefined}
          onRecovery={catalog?.recovery ? () => open({ kind: 'recovery', from: modal }) : undefined}
          onSwitch={() => open({ ...modal, mode: modal.mode === 'login' ? 'register' : 'login' })} />}
        {modal.kind === 'recovery' && <RecoveryForm key="recovery" sent={modal.sent} busy={busy} error={error} onSubmit={sendRecovery} onBack={() => open(modal.from)} />}
        {modal.kind === 'reset' && <RecoveryForm key="reset" reset busy={busy} error={error} onSubmit={resetPassword} onBack={openAccount} />}
        {modal.kind === 'booking' && <BookingCalendar catalog={catalog} catalogError={catalogError} selection={selection} account={account} busy={busy} error={error} reschedule={modal.reschedule}
          onChange={(key, value) => setSelection(current => {
            if (key === 'slotId') return { ...current, slotId: value }
            return { ...current, [key]: value, slotId: '', moduleId: key === 'language' ? '' : key === 'moduleId' ? value : current.moduleId }
          })}
          onReload={() => reloadCatalog()} onContinue={continueBooking} />}
        {modal.kind === 'module' && school && <ModuleDetails item={displayedModule} school={school} account={account} onPrepare={onPrepare} onBook={onBook} busy={busy} error={error} />}
        {modal.kind === 'plan' && <ActivatePlan plan={modal.plan} account={account} busy={busy} error={error} onSubmit={activatePlan} mode={catalog?.payments} school={school} />}
        {modal.kind === 'booked' && <Booked booking={modal.booking} onCabinet={() => showCabinet('lessons')} />}
        {modal.kind === 'cancel-booking' && <Confirm text="Место освободится. Если урок списан с действующего пакета, он вернётся на баланс." busy={busy} error={error} onSubmit={confirmCancellation} />}
        {modal.kind === 'cancel-plan' && <Confirm text="Оставшиеся уроки станут недоступны. Подтверждённые записи сохранятся." busy={busy} error={error} onSubmit={confirmCancellation} />}
      </Dialog>}
    </>
  )
}

createRoot(document.getElementById('root')).render(<App />)
