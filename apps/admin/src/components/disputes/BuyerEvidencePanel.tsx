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
          <User className="h-5 w-5 text-blue-600" />
          Buyer Evidence
          <span className="text-sm font-normal text-gray-500">
            {dispute.buyer_username || 'Unknown Buyer'}
          </span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Buyer Statement */}
        {hasStatement && (
          <div className="bg-blue-50 rounded-lg p-4 border border-blue-100">
            <div className="flex items-center gap-2 mb-2">
              <MessageSquare className="h-4 w-4 text-blue-600" />
              <p className="text-sm font-medium text-blue-900">Buyer Statement</p>
            </div>
            <p className="text-sm text-gray-900 whitespace-pre-wrap">
              {buyerEvidence?.statement}
            </p>
          </div>
        )}

        {/* Evidence Items */}
        {hasEvidence ? (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              {displayEvidence.length} evidence item{displayEvidence.length !== 1 ? 's' : ''} submitted
            </p>

            {displayEvidence.map((item) => {
              const isExpanded = expandedEvidence.has(item.id)
              const isImage = item.type === 'image'

              return (
                <div
                  key={item.id}
                  className="border border-gray-200 rounded-lg overflow-hidden"
                >
                  {/* Evidence Header */}
                  <div
                    className="flex items-center justify-between p-3 bg-gray-50 cursor-pointer hover:bg-gray-100 transition-colors"
                    onClick={() => toggleExpanded(item.id)}
                  >
                    <div className="flex items-center gap-3">
                      {isImage ? (
                        <FileImage className="h-4 w-4 text-gray-600" />
                      ) : (
                        <FileText className="h-4 w-4 text-gray-600" />
                      )}
                      <div>
                        <p className="text-sm font-medium text-gray-900">
                          {item.description || `Evidence ${item.id.slice(-6)}`}
                        </p>
                        <p className="text-xs text-gray-500">
                          {formatDate(item.submitted_at)}
                        </p>
                      </div>
                    </div>
                    {isExpanded ? (
                      <ChevronDown className="h-4 w-4 text-gray-500" />
                    ) : (
                      <ChevronRight className="h-4 w-4 text-gray-500" />
                    )}
                  </div>

                  {/* Evidence Content */}
                  {isExpanded && (
                    <div className="p-3 border-t border-gray-200">
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
                            className="w-full max-h-80 object-contain rounded-lg bg-gray-100"
                          />
                        </a>
                      ) : item.type === 'document' && item.url ? (
                        <a
                          href={item.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-2 text-blue-600 hover:text-blue-700"
                        >
                          <FileText className="h-4 w-4" />
                          <span className="text-sm">Open Document</span>
                        </a>
                      ) : item.content ? (
                        <div className="bg-gray-50 rounded-lg p-3">
                          <p className="text-sm text-gray-900 whitespace-pre-wrap">
                            {item.content}
                          </p>
                        </div>
                      ) : (
                        <p className="text-sm text-gray-500">No content available</p>
                      )}
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        ) : (
          <div className="text-center py-8 bg-gray-50 rounded-lg">
            <FileImage className="h-10 w-10 text-gray-400 mx-auto mb-2" />
            <p className="text-sm text-gray-600">No evidence submitted by buyer</p>
          </div>
        )}
      </CardContent>
    </Card>
  )
}
