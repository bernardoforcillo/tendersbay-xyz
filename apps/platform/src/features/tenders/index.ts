export { CitationRef, type CitationRefProps } from './components/atoms';
export {
  CoverageBadge,
  type CoverageBadgeProps,
  EvidenceQuote,
  type EvidenceQuoteProps,
} from './components/molecules';
export {
  AwardCriteriaGrid,
  type AwardCriteriaGridProps,
  TenderCoverage,
  type TenderCoverageProps,
} from './components/organisms';
export type { TenderDetailViewProps } from './components/templates';
export { TenderDetailView } from './components/templates';
export {
  type AwardCriterionView,
  type CitationView,
  COVERAGE_REASONS,
  COVERAGE_VALUES,
  type Coverage,
  type CoverageReason,
  CRITERION_TYPES,
  type CriterionType,
  citationFromRequirement,
  type DocumentAvailabilityView,
  type PassageView,
  type TenderCriteriaSource,
} from './constants';
export {
  type TenderPassagesResult,
  useTenderAvailability,
  useTenderDetail,
  useTenderPassages,
} from './hooks';
export type { TenderDetailData } from './load-tender-detail';
export { loadTenderDetail, TenderNotFoundError } from './load-tender-detail';
export { useTenderHead } from './use-tender-head';
export { useTenderLink } from './use-tender-link';
