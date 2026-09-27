import { useState } from 'react'
import { Store, FileImage, FileText, MessageSquare, ChevronDown, ChevronRight } from 'lucide-react'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/Card'
import type { DisputeDetail, PartyEvidence, EvidenceItem } from '@/types'
import { formatDate } from '@/lib/utils'

interface SellerEvidencePanelProps {
  dispute: DisputeDetail
}

export function SellerEvidencePanel({ dispute }: SellerEvidencePanelProps) {
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

  // Get seller evidence
  const sellerEvidence: PartyEvidence | null = dispute.seller_evidence || null

  // Get all evidence items
  const allEvidence: EvidenceItem[] = sellerEvidence?.evidence || []

  const displayEvidence = allEvidence

  const hasEvidence = displayEvidence.length > 0
  const hasStatement = !!(sellerEvidence?.statement)

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg flex items-center gap-2">
          <Store className="h-5 w-5 text-success" />
          Seller Evidence
          <span className="text-sm font-normal text-muted-foreground">
            {dispute.seller_username || 'Unknown Seller'}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Seller Statement */}
        {hasStatement && (
          <div className="bg-success-bg rounded-lg p-4 border border-success">
            <div className="flex items-center gap-2 mb-2">
              <MessageSquare className="h-4 w-4 text-success" />
              <p className="text-sm font-medium text-success">Seller Response</p>
            </div>
            <p className="text-sm text-foreground whitespace-pre-wrap">
              {sellerEvidence?.statement}
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
            <p className="text-sm text-muted-foreground">No evidence submitted by seller</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
