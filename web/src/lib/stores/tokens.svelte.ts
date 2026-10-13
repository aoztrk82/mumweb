// Access tokens = channel passwords (Mumble "access tokens"). Tokens entered in this
// session are sent to the server; "remembered" ones are stored locally and sent at login.
import { SvelteSet } from 'svelte/reactivity'
import { getSetting, setSetting } from '../settings'

class Tokens {
  remembered = new SvelteSet<string>(getSetting<string[]>('accessTokens', []))
  session = new SvelteSet<string>(this.remembered)

  all(): string[] {
    return [...this.session]
  }

  add(token: string, remember: boolean): void {
    this.session.add(token)
    if (remember) {
      this.remembered.add(token)
      this.save()
    }
  }

  remove(token: string): void {
    this.session.delete(token)
    if (this.remembered.delete(token)) this.save()
  }

  private save(): void {
    setSetting('accessTokens', [...this.remembered])
  }
}

export const tokens = new Tokens()
