import { useState } from 'react'
import { User, FileImage, FileText, MessageSquare, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import type { DisputeDetail, PartyEvidence, EvidenceItem } from '@/types'
import { formatDate } from '@/lib/utils'

interface BuyerEvidencePanelProps {
  dispute: DisputeDetail
}

export function BuyerEvidencePanel({ dispute }: BuyerEvidencePanelProps) {
  const [expandedEvidence, setExpandedEvidence] = useState<Set<string>>(new Set())

  const toggleExpanded = (id: string) => {
    setExpandedEvidence(prev => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  // Get buyer evidence
  const buyerEvidence: PartyEvidence | null = dispute.buyer_evidence || null

  const allEvidence: EvidenceItem[] = buyerEvidence?.evidence || []
  const evidenceUrls = dispute.evidence || []
  const displayEvidence: EvidenceItem[] = allEvidence.length > 0
    ? allEvidence
    : evidenceUrls.map((url, idx) => ({
        id: `url-${idx}`,
        type: 'image' as const,
        url,
        submitted_by: 'buyer' as const,
        submitted_at: dispute.opened_at,
      })) as EvidenceItem[]

  const hasEvidence = displayEvidence.length > 0
  const hasStatement = !!(buyerEvidence?.statement)

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <User className="h-5 w-5 text-primary" />
          Buyer Evidence
          <span className="text-sm font-normal text-muted-foreground">
            {dispute.buyer_username || 'Unknown Buyer'}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Buyer Statement */}
        {hasStatement && (
          <div className="bg-primary/10 rounded-lg p-4 border border-primary/20">
            <div className="flex items-center gap-2 mb-2">
              <MessageSquare className="h-4 w-4 text-primary" />
              <p className="text-sm font-medium text-primary">Buyer Statement</p>
            </div>
            <p className="text-sm text-foreground whitespace-pre-wrap">
              {buyerEvidence?.statement}
            </p>
          </div>
        )}

        {/* Evidence Items */}
        {hasEvidence ? (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              {displayEvidence.length} evidence item{displayEvidence.length !== 1 ? 's' : ''} submitted
            </p>

            {displayEvidence.map((item) => {
              const isExpanded = expandedEvidence.has(item.id)
              const isImage = item.type === 'image'

              return (
                <div
                  key={item.id}
                  className="border border-border rounded-lg overflow-hidden"
                >
                  {/* Evidence Header */}
                  <div
                    className="flex items-center justify-between p-3 bg-muted cursor-pointer hover:bg-muted transition-colors"
                    onClick={() => toggleExpanded(item.id)}
                  >
                    <div className="flex items-center gap-3">
                      {isImage ? (
                        <FileImage className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <FileText className="h-4 w-4 text-muted-foreground" />
                      )}
                      <div>
                        <p className="text-sm font-medium text-foreground">
                          {item.description || `Evidence ${item.id.slice(-6)}`}
                        </p>
                        <p className="text-xs text-muted-foreground">
                          {formatDate(item.submitted_at)}
                        </p>
                      </div>
                    </div>
                    {isExpanded ? (
                      <ChevronDown className="h-4 w-4 text-muted-foreground" />
                    ) : (
                      <ChevronRight className="h-4 w-4 text-muted-foreground" />
                    )}
                  </div>

                  {/* Evidence Content */}
                  {isExpanded && (
                    <div className="p-3 border-t border-border">
                      {item.type === 'image' && item.url ? (
                        <a
                          href={item.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="block"
                        >
                          <img
                            src={item.url}
                            alt={item.description || 'Evidence'}
                            className="w-full max-h-80 object-contain rounded-lg bg-muted"
                          />
                        </a>
                      ) : item.type === 'document' && item.url ? (
                        <a
                          href={item.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-2 text-primary hover:text-primary"
                        >
                          <FileText className="h-4 w-4" />
                          <span className="text-sm">Open Document</span>
                        </a>
                      ) : item.content ? (
                        <div className="bg-muted rounded-lg p-3">
                          <p className="text-sm text-foreground whitespace-pre-wrap">
                            {item.content}
                          </p>
                        </div>
                      ) : (
                        <p className="text-sm text-muted-foreground">No content available</p>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        ) : (
          <div className="text-center py-8 bg-muted rounded-lg">
            <FileImage className="h-10 w-10 text-muted-foreground mx-auto mb-2" />
            <p className="text-sm text-muted-foreground">No evidence submitted by buyer</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
