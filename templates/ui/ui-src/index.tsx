import type { Host, PluginModule, PluginSettingsContext } from '@splify2/plugin-api'
import { useState } from 'preact/hooks'
import './style.css'

function Settings({ host, ctx }: { host: Host; ctx: PluginSettingsContext }) {
  const { Card, CardContent, CardHead, Button, KV } = host.ui
  const [answer, setAnswer] = useState<string | null>(null)

  const ask = async () => {
    try {
      // метод hello из rpc.methods манифеста: rpc/hello получает параметры JSON в stdin
      const r = await host.rpc.call<{ message: string }>('hello', { from: host.plugin.id })
      setAnswer(r.message)
    } catch (e) {
      host.notify((e as Error).message, 'error')
    }
  }

  return (
    <Card>
      <CardContent>
        <CardHead title={host.i18n.t('title')} />
        <KV k={host.i18n.t('version')} v={host.plugin.version} />
        <Button onClick={ask}>{host.i18n.t('ask')}</Button>{' '}
        <Button variant="ghost" onClick={ctx.close}>{host.i18n.t('close')}</Button>
        {answer && <p class="example-ui__answer">{answer}</p>}
      </CardContent>
    </Card>
  )
}

const plugin: PluginModule = {
  activate(host) {
    host.slots.pluginSettings({
      summary: () => ({ text: host.i18n.t('summary'), tone: 'ok' }),
      render: ctx => <Settings host={host} ctx={ctx} />,
    })
  },
}

export default plugin
