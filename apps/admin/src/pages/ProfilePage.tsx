import { Shield, User } from 'lucide-react'
import { Avatar } from '@/components/ui/Avatar'
import { Badge } from '@/components/ui/Badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import { useAuth } from '@/hooks/useAuth'
import { formatCapability } from '@/lib/permissions'

export function ProfilePage() {
  const { user, capabilities } = useAuth()

  if (!user) return null

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold text-foreground">My Profile</h1>
        <p className="mt-1 text-muted-foreground">Your account identity and admin authority</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5 text-primary" />
            Account Information
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center gap-4">
            <Avatar src={user.avatarUrl} userId={user.username || user.email} name={user.username || user.email} size="lg" />
            <div>
              <p className="text-lg font-semibold text-foreground">@{user.username || 'Admin'}</p>
              <p className="text-sm text-muted-foreground">{user.email || 'No email available'}</p>
            </div>
          </div>
          <dl className="mt-6 grid gap-4 sm:grid-cols-2">
            <div>
              <dt className="text-sm text-muted-foreground">User ID</dt>
              <dd className="mt-1 break-all font-mono text-sm text-foreground">{user.id}</dd>
            </div>
            <div>
              <dt className="text-sm text-muted-foreground">Admin status</dt>
              <dd className="mt-1"><Badge variant="success">{user.isAdmin ? 'Active admin' : 'Not admin'}</Badge></dd>
            </div>
          </dl>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5 text-primary" />
            Assigned Capabilities
          </CardTitle>
        </CardHeader>
        <CardContent>
          {capabilities.length ? (
            <div className="grid gap-3 sm:grid-cols-2">
              {capabilities.map((capability) => (
                <div key={capability} className="rounded-lg border border-border p-3">
                  <p className="text-sm font-medium text-foreground">{formatCapability(capability)}</p>
                  <p className="mt-1 font-mono text-xs text-muted-foreground">{capability}</p>
                </div>
              ))}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">No capabilities assigned.</p>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
