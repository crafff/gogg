import type * as Operations from "./types";
import { DocumentTypeDecoration } from '@graphql-typed-document-node/core';
import { useQuery, useMutation, UseQueryOptions, UseMutationOptions } from '@tanstack/react-query';
import { fetcher } from '../fetcher';

export class TypedDocumentString<TResult, TVariables>
  extends String
  implements DocumentTypeDecoration<TResult, TVariables>
{
  __apiType?: NonNullable<DocumentTypeDecoration<TResult, TVariables>['__apiType']>;
  private value: string;
  public __meta__?: Record<string, any> | undefined;

  constructor(value: string, __meta__?: Record<string, any> | undefined) {
    super(value);
    this.value = value;
    this.__meta__ = __meta__;
  }

  override toString(): string & DocumentTypeDecoration<TResult, TVariables> {
    return this.value;
  }
}

export const MeDocument = new TypedDocumentString(`
    query Me {
  me {
    id
    displayName
    email
    avatarUrl
    locale
    identities {
      provider
      username
    }
  }
}
    `);

export const useMeQuery = <
      TData = Operations.MeQuery,
      TError = unknown
    >(
      variables?: Operations.MeQueryVariables,
      options?: Omit<UseQueryOptions<Operations.MeQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.MeQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.MeQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['Me'] : ['Me', variables],
    queryFn: fetcher<Operations.MeQuery, Operations.MeQueryVariables>(MeDocument, variables),
    ...options
  }
    )};

useMeQuery.getKey = (variables?: Operations.MeQueryVariables) => variables === undefined ? ['Me'] : ['Me', variables];


useMeQuery.fetcher = (variables?: Operations.MeQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.MeQuery, Operations.MeQueryVariables>(MeDocument, variables, options);

export const AuthProvidersDocument = new TypedDocumentString(`
    query AuthProviders {
  authProviders {
    id
  }
}
    `);

export const useAuthProvidersQuery = <
      TData = Operations.AuthProvidersQuery,
      TError = unknown
    >(
      variables?: Operations.AuthProvidersQueryVariables,
      options?: Omit<UseQueryOptions<Operations.AuthProvidersQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.AuthProvidersQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.AuthProvidersQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['AuthProviders'] : ['AuthProviders', variables],
    queryFn: fetcher<Operations.AuthProvidersQuery, Operations.AuthProvidersQueryVariables>(AuthProvidersDocument, variables),
    ...options
  }
    )};

useAuthProvidersQuery.getKey = (variables?: Operations.AuthProvidersQueryVariables) => variables === undefined ? ['AuthProviders'] : ['AuthProviders', variables];


useAuthProvidersQuery.fetcher = (variables?: Operations.AuthProvidersQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.AuthProvidersQuery, Operations.AuthProvidersQueryVariables>(AuthProvidersDocument, variables, options);

export const VersionsDocument = new TypedDocumentString(`
    query Versions {
  versions
}
    `);

export const useVersionsQuery = <
      TData = Operations.VersionsQuery,
      TError = unknown
    >(
      variables?: Operations.VersionsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.VersionsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.VersionsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.VersionsQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['Versions'] : ['Versions', variables],
    queryFn: fetcher<Operations.VersionsQuery, Operations.VersionsQueryVariables>(VersionsDocument, variables),
    ...options
  }
    )};

useVersionsQuery.getKey = (variables?: Operations.VersionsQueryVariables) => variables === undefined ? ['Versions'] : ['Versions', variables];


useVersionsQuery.fetcher = (variables?: Operations.VersionsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.VersionsQuery, Operations.VersionsQueryVariables>(VersionsDocument, variables, options);

export const RegionsDocument = new TypedDocumentString(`
    query Regions {
  regions
}
    `);

export const useRegionsQuery = <
      TData = Operations.RegionsQuery,
      TError = unknown
    >(
      variables?: Operations.RegionsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.RegionsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.RegionsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.RegionsQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['Regions'] : ['Regions', variables],
    queryFn: fetcher<Operations.RegionsQuery, Operations.RegionsQueryVariables>(RegionsDocument, variables),
    ...options
  }
    )};

useRegionsQuery.getKey = (variables?: Operations.RegionsQueryVariables) => variables === undefined ? ['Regions'] : ['Regions', variables];


useRegionsQuery.fetcher = (variables?: Operations.RegionsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.RegionsQuery, Operations.RegionsQueryVariables>(RegionsDocument, variables, options);

export const ChampionDetailDocument = new TypedDocumentString(`
    query ChampionDetail($id: Int!, $filter: ChampionDetailFilter) {
  championDetail(id: $id, filter: $filter) {
    championId
    championName
    games
    resolvedVersion
    runeBuilds {
      primaryStyleId
      secondaryStyleId
      perkIds
      statShardIds
      games
      eligibleGames
      pickRate
      winRate
    }
    summonerSpellBuilds {
      ids
      games
      eligibleGames
      pickRate
      winRate
    }
    starterBuilds {
      ids
      games
      eligibleGames
      pickRate
      winRate
    }
    bootsBuilds {
      ids
      games
      eligibleGames
      pickRate
      winRate
    }
    itemBuilds {
      stage
      builds {
        ids
        games
        eligibleGames
        pickRate
        winRate
      }
    }
  }
}
    `);

export const useChampionDetailQuery = <
      TData = Operations.ChampionDetailQuery,
      TError = unknown
    >(
      variables: Operations.ChampionDetailQueryVariables,
      options?: Omit<UseQueryOptions<Operations.ChampionDetailQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.ChampionDetailQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.ChampionDetailQuery, TError, TData>(
      {
    queryKey: ['ChampionDetail', variables],
    queryFn: fetcher<Operations.ChampionDetailQuery, Operations.ChampionDetailQueryVariables>(ChampionDetailDocument, variables),
    ...options
  }
    )};

useChampionDetailQuery.getKey = (variables: Operations.ChampionDetailQueryVariables) => ['ChampionDetail', variables];


useChampionDetailQuery.fetcher = (variables: Operations.ChampionDetailQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.ChampionDetailQuery, Operations.ChampionDetailQueryVariables>(ChampionDetailDocument, variables, options);

export const ChampionWinFactorsDocument = new TypedDocumentString(`
    query ChampionWinFactors($id: Int!, $filter: ChampionWinFactorsFilter!) {
  championWinFactors(id: $id, filter: $filter) {
    championId
    championName
    position
    resolvedVersion
    regionScope
    tierGroup
    revision
    algorithm
    dataThrough
    publishedAt
    availability
    unavailableReason
    cohortScope
    sampleGames
    samplePlayers
    factors {
      metricKey
      kind
      startMinute
      endMinute
      unit
      p50
      p70
      p90
      evidenceGrade
      displayOrder
      buckets {
        ordinal
        lowerBound
        upperBound
        games
        wins
        samplePlayers
        observedWinRate
        observedWinRateDelta
      }
    }
  }
}
    `);

export const useChampionWinFactorsQuery = <
      TData = Operations.ChampionWinFactorsQuery,
      TError = unknown
    >(
      variables: Operations.ChampionWinFactorsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.ChampionWinFactorsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.ChampionWinFactorsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.ChampionWinFactorsQuery, TError, TData>(
      {
    queryKey: ['ChampionWinFactors', variables],
    queryFn: fetcher<Operations.ChampionWinFactorsQuery, Operations.ChampionWinFactorsQueryVariables>(ChampionWinFactorsDocument, variables),
    ...options
  }
    )};

useChampionWinFactorsQuery.getKey = (variables: Operations.ChampionWinFactorsQueryVariables) => ['ChampionWinFactors', variables];


useChampionWinFactorsQuery.fetcher = (variables: Operations.ChampionWinFactorsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.ChampionWinFactorsQuery, Operations.ChampionWinFactorsQueryVariables>(ChampionWinFactorsDocument, variables, options);

export const ChampionRankingsDocument = new TypedDocumentString(`
    query ChampionRankings($filter: ChampionRankingsFilter) {
  championRankings(filter: $filter) {
    items {
      championId
      championName
      teamPosition
      games
      wins
      losses
      winRate
      pickRate
      banRate
      kda
    }
    totalMatches
    resolvedVersion
  }
}
    `);

export const useChampionRankingsQuery = <
      TData = Operations.ChampionRankingsQuery,
      TError = unknown
    >(
      variables?: Operations.ChampionRankingsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.ChampionRankingsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.ChampionRankingsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.ChampionRankingsQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['ChampionRankings'] : ['ChampionRankings', variables],
    queryFn: fetcher<Operations.ChampionRankingsQuery, Operations.ChampionRankingsQueryVariables>(ChampionRankingsDocument, variables),
    ...options
  }
    )};

useChampionRankingsQuery.getKey = (variables?: Operations.ChampionRankingsQueryVariables) => variables === undefined ? ['ChampionRankings'] : ['ChampionRankings', variables];


useChampionRankingsQuery.fetcher = (variables?: Operations.ChampionRankingsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.ChampionRankingsQuery, Operations.ChampionRankingsQueryVariables>(ChampionRankingsDocument, variables, options);

export const SummonerDocument = new TypedDocumentString(`
    query Summoner($identity: SummonerIdentityInput!, $history: SummonerHistoryInput) {
  summoner(identity: $identity, history: $history) {
    profile {
      region
      gameName
      tagLine
      profileIconId
      summonerLevel
      lastRefreshedAt
      isStale
    }
    ranks {
      queueType
      tier
      division
      leaguePoints
      wins
      losses
      winRate
    }
    history {
      items {
        matchId
        queue
        queueId
        gameStartTime
        durationSeconds
        version
        endOfGameResult
        position
        win
        championId
        championName
        championLevel
        kills
        deaths
        assists
        kda
        minionsKilled
        csPerMinute
        goldEarned
        damageToChampions
        visionScore
        itemIds
        summonerSpellIds
        primaryStyleId
        secondaryStyleId
        perkIds
        statShardIds
        earlySurrender
        surrender
        averageTier
        averageDivision
        tierCoverage
        participants {
          participantId
          teamId
          isCurrentPlayer
          gameName
          tagLine
          position
          win
          championId
          championName
          championLevel
          kills
          deaths
          assists
          kda
          minionsKilled
          goldEarned
          damageToChampions
          visionScore
          itemIds
          summonerSpellIds
          primaryStyleId
          secondaryStyleId
          perkIds
          rank {
            tier
            division
            leaguePoints
            snapshotDeltaHours
          }
        }
      }
      pageInfo {
        endCursor
        hasNextPage
        returned
      }
    }
  }
}
    `);

export const useSummonerQuery = <
      TData = Operations.SummonerQuery,
      TError = unknown
    >(
      variables: Operations.SummonerQueryVariables,
      options?: Omit<UseQueryOptions<Operations.SummonerQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.SummonerQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.SummonerQuery, TError, TData>(
      {
    queryKey: ['Summoner', variables],
    queryFn: fetcher<Operations.SummonerQuery, Operations.SummonerQueryVariables>(SummonerDocument, variables),
    ...options
  }
    )};

useSummonerQuery.getKey = (variables: Operations.SummonerQueryVariables) => ['Summoner', variables];


useSummonerQuery.fetcher = (variables: Operations.SummonerQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.SummonerQuery, Operations.SummonerQueryVariables>(SummonerDocument, variables, options);

export const RefreshSummonerDocument = new TypedDocumentString(`
    mutation RefreshSummoner($identity: SummonerIdentityInput!) {
  refreshSummoner(identity: $identity) {
    fresh
    reused
    job {
      id
      region
      gameName
      tagLine
      status
      stage
      scannedCount
      supportedCount
      fetchedCount
      failedCount
      errorCode
      errorMessage
      createdAt
      updatedAt
      completedAt
    }
  }
}
    `);

export const useRefreshSummonerMutation = <
      TError = unknown,
      TContext = unknown
    >(options?: UseMutationOptions<Operations.RefreshSummonerMutation, TError, Operations.RefreshSummonerMutationVariables, TContext>) => {

    return useMutation<Operations.RefreshSummonerMutation, TError, Operations.RefreshSummonerMutationVariables, TContext>(
      {
    mutationKey: ['RefreshSummoner'],
    mutationFn: (variables?: Operations.RefreshSummonerMutationVariables) => fetcher<Operations.RefreshSummonerMutation, Operations.RefreshSummonerMutationVariables>(RefreshSummonerDocument, variables)(),
    ...options
  }
    )};


useRefreshSummonerMutation.fetcher = (variables: Operations.RefreshSummonerMutationVariables, options?: RequestInit['headers']) => fetcher<Operations.RefreshSummonerMutation, Operations.RefreshSummonerMutationVariables>(RefreshSummonerDocument, variables, options);

export const SummonerLookupJobDocument = new TypedDocumentString(`
    query SummonerLookupJob($id: ID!) {
  summonerLookupJob(id: $id) {
    id
    region
    gameName
    tagLine
    status
    stage
    scannedCount
    supportedCount
    fetchedCount
    failedCount
    errorCode
    errorMessage
    createdAt
    updatedAt
    completedAt
  }
}
    `);

export const useSummonerLookupJobQuery = <
      TData = Operations.SummonerLookupJobQuery,
      TError = unknown
    >(
      variables: Operations.SummonerLookupJobQueryVariables,
      options?: Omit<UseQueryOptions<Operations.SummonerLookupJobQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.SummonerLookupJobQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.SummonerLookupJobQuery, TError, TData>(
      {
    queryKey: ['SummonerLookupJob', variables],
    queryFn: fetcher<Operations.SummonerLookupJobQuery, Operations.SummonerLookupJobQueryVariables>(SummonerLookupJobDocument, variables),
    ...options
  }
    )};

useSummonerLookupJobQuery.getKey = (variables: Operations.SummonerLookupJobQueryVariables) => ['SummonerLookupJob', variables];


useSummonerLookupJobQuery.fetcher = (variables: Operations.SummonerLookupJobQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.SummonerLookupJobQuery, Operations.SummonerLookupJobQueryVariables>(SummonerLookupJobDocument, variables, options);

export const TftAnalysisCatalogDocument = new TypedDocumentString(`
    query TFTAnalysisCatalog {
  tftAnalysisCatalog {
    platform
    patch
    setNumber
    queueId
    cohort
    window
    publishedAt
    sourceMatches
    sourceParticipants
    coverage {
      windowStart
      windowEnd
      exactLineups
      familyThreshold
    }
  }
}
    `);

export const useTftAnalysisCatalogQuery = <
      TData = Operations.TftAnalysisCatalogQuery,
      TError = unknown
    >(
      variables?: Operations.TftAnalysisCatalogQueryVariables,
      options?: Omit<UseQueryOptions<Operations.TftAnalysisCatalogQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.TftAnalysisCatalogQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.TftAnalysisCatalogQuery, TError, TData>(
      {
    queryKey: variables === undefined ? ['TFTAnalysisCatalog'] : ['TFTAnalysisCatalog', variables],
    queryFn: fetcher<Operations.TftAnalysisCatalogQuery, Operations.TftAnalysisCatalogQueryVariables>(TftAnalysisCatalogDocument, variables),
    ...options
  }
    )};

useTftAnalysisCatalogQuery.getKey = (variables?: Operations.TftAnalysisCatalogQueryVariables) => variables === undefined ? ['TFTAnalysisCatalog'] : ['TFTAnalysisCatalog', variables];


useTftAnalysisCatalogQuery.fetcher = (variables?: Operations.TftAnalysisCatalogQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.TftAnalysisCatalogQuery, Operations.TftAnalysisCatalogQueryVariables>(TftAnalysisCatalogDocument, variables, options);

export const TftLineupsDocument = new TypedDocumentString(`
    query TFTLineups($filter: TFTLineupsFilter!) {
  tftLineups(filter: $filter) {
    platform
    patch
    setNumber
    queueId
    cohort
    window
    locale
    algorithmVersion
    publishedAt
    sourceMatches
    sourceParticipants
    coverage {
      windowStart
      windowEnd
      exactLineups
      familyThreshold
    }
    items {
      id
      coreUnits {
        id
        name
        iconUrl
      }
      commonItems {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      commonAugments {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      commonTraits {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      unitItems {
        unit {
          id
          name
          iconUrl
        }
        commonItems {
          entity {
            id
            name
            iconUrl
          }
          count
          rate
        }
        isCore
        coreRank
        averageItems
        itemInvestmentRate
        equippedRate
        threeItemRate
        knownStarSamples
        unknownStarSamples
        starCoverage
        starDistribution {
          stars
          sampleSize
          rate
          knownRate
          avgPlacement
          firstRate
          top4Rate
        }
      }
      starLevels {
        totalStars
        sampleSize
        lobbyCount
        rate
        avgPlacement
        firstRate
        top4Rate
      }
      starCompositionKnownSamples
      starCompositionUnknownSamples
      starCompositionCoverage
      starCompositions {
        levels {
          stars
          unitCount
        }
        totalStars
        sampleSize
        rate
        avgPlacement
        firstRate
        top4Rate
      }
      metrics {
        sampleSize
        lobbyCount
        pickRate
        avgPlacement
        firstRate
        top4Rate
        contestedRate
      }
    }
  }
}
    `);

export const useTftLineupsQuery = <
      TData = Operations.TftLineupsQuery,
      TError = unknown
    >(
      variables: Operations.TftLineupsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.TftLineupsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.TftLineupsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.TftLineupsQuery, TError, TData>(
      {
    queryKey: ['TFTLineups', variables],
    queryFn: fetcher<Operations.TftLineupsQuery, Operations.TftLineupsQueryVariables>(TftLineupsDocument, variables),
    ...options
  }
    )};

useTftLineupsQuery.getKey = (variables: Operations.TftLineupsQueryVariables) => ['TFTLineups', variables];


useTftLineupsQuery.fetcher = (variables: Operations.TftLineupsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.TftLineupsQuery, Operations.TftLineupsQueryVariables>(TftLineupsDocument, variables, options);

export const TftObservedLineupsDocument = new TypedDocumentString(`
    query TFTObservedLineups($filter: TFTObservedLineupsFilter!) {
  tftObservedLineups(filter: $filter) {
    dataKind
    runId
    platform
    platforms
    queueId
    setNumber
    patch
    rawGameVersions
    locale
    algorithmVersion
    catalogSnapshot {
      source
      patch
      revision
    }
    assetSnapshot {
      source
      patch
      revision
    }
    sourceMatches
    sourceParticipants
    usableParticipants
    exactLineups
    windowStart
    windowEnd
    items {
      id
      coreUnits {
        id
        name
        iconUrl
      }
      commonItems {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      commonAugments {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      commonTraits {
        entity {
          id
          name
          iconUrl
        }
        count
        rate
      }
      unitItems {
        unit {
          id
          name
          iconUrl
        }
        commonItems {
          entity {
            id
            name
            iconUrl
          }
          count
          rate
        }
        isCore
        coreRank
        averageItems
        itemInvestmentRate
        equippedRate
        threeItemRate
        knownStarSamples
        unknownStarSamples
        starCoverage
        starDistribution {
          stars
          sampleSize
          rate
          knownRate
          avgPlacement
          firstRate
          top4Rate
        }
      }
      starLevels {
        totalStars
        sampleSize
        lobbyCount
        rate
        avgPlacement
        firstRate
        top4Rate
      }
      starCompositionKnownSamples
      starCompositionUnknownSamples
      starCompositionCoverage
      starCompositions {
        levels {
          stars
          unitCount
        }
        totalStars
        sampleSize
        rate
        avgPlacement
        firstRate
        top4Rate
      }
      metrics {
        sampleSize
        lobbyCount
        pickRate
        avgPlacement
        firstRate
        top4Rate
        contestedRate
      }
    }
  }
}
    `);

export const useTftObservedLineupsQuery = <
      TData = Operations.TftObservedLineupsQuery,
      TError = unknown
    >(
      variables: Operations.TftObservedLineupsQueryVariables,
      options?: Omit<UseQueryOptions<Operations.TftObservedLineupsQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.TftObservedLineupsQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.TftObservedLineupsQuery, TError, TData>(
      {
    queryKey: ['TFTObservedLineups', variables],
    queryFn: fetcher<Operations.TftObservedLineupsQuery, Operations.TftObservedLineupsQueryVariables>(TftObservedLineupsDocument, variables),
    ...options
  }
    )};

useTftObservedLineupsQuery.getKey = (variables: Operations.TftObservedLineupsQueryVariables) => ['TFTObservedLineups', variables];


useTftObservedLineupsQuery.fetcher = (variables: Operations.TftObservedLineupsQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.TftObservedLineupsQuery, Operations.TftObservedLineupsQueryVariables>(TftObservedLineupsDocument, variables, options);

export const TftMatchHistoryDocument = new TypedDocumentString(`
    query TFTMatchHistory($input: TFTHistoryInput!) {
  tftMatchHistory(input: $input) {
    profile {
      platform
      gameName
      tagLine
      lastRefreshedAt
      isStale
    }
    matches {
      matchId
      platform
      queueId
      patch
      gameVersion
      gameDatetime
      gameLengthSeconds
      setNumber
      tftGameType
      endOfGameResult
      participant {
        puuid
        gameName
        tagLine
        isCurrentPlayer
        placement
        level
        goldLeft
        lastRound
        playersEliminated
        totalDamageToPlayers
        augments {
          id
          name
          iconUrl
        }
        traits {
          entity {
            id
            name
            iconUrl
          }
          numUnits
          style
          tierCurrent
          tierTotal
        }
        units {
          entity {
            id
            name
            iconUrl
          }
          rarity
          tier
          items {
            id
            name
            iconUrl
          }
        }
      }
    }
    pageInfo {
      endCursor
      hasNextPage
      returned
    }
  }
}
    `);

export const useTftMatchHistoryQuery = <
      TData = Operations.TftMatchHistoryQuery,
      TError = unknown
    >(
      variables: Operations.TftMatchHistoryQueryVariables,
      options?: Omit<UseQueryOptions<Operations.TftMatchHistoryQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.TftMatchHistoryQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.TftMatchHistoryQuery, TError, TData>(
      {
    queryKey: ['TFTMatchHistory', variables],
    queryFn: fetcher<Operations.TftMatchHistoryQuery, Operations.TftMatchHistoryQueryVariables>(TftMatchHistoryDocument, variables),
    ...options
  }
    )};

useTftMatchHistoryQuery.getKey = (variables: Operations.TftMatchHistoryQueryVariables) => ['TFTMatchHistory', variables];


useTftMatchHistoryQuery.fetcher = (variables: Operations.TftMatchHistoryQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.TftMatchHistoryQuery, Operations.TftMatchHistoryQueryVariables>(TftMatchHistoryDocument, variables, options);

export const RefreshTftPlayerDocument = new TypedDocumentString(`
    mutation RefreshTFTPlayer($identity: TFTIdentityInput!) {
  refreshTFTPlayer(identity: $identity) {
    fresh
    reused
    job {
      id
      platform
      gameName
      tagLine
      status
      stage
      scannedCount
      fetchedCount
      failedCount
      errorCode
      createdAt
      updatedAt
      completedAt
    }
  }
}
    `);

export const useRefreshTftPlayerMutation = <
      TError = unknown,
      TContext = unknown
    >(options?: UseMutationOptions<Operations.RefreshTftPlayerMutation, TError, Operations.RefreshTftPlayerMutationVariables, TContext>) => {

    return useMutation<Operations.RefreshTftPlayerMutation, TError, Operations.RefreshTftPlayerMutationVariables, TContext>(
      {
    mutationKey: ['RefreshTFTPlayer'],
    mutationFn: (variables?: Operations.RefreshTftPlayerMutationVariables) => fetcher<Operations.RefreshTftPlayerMutation, Operations.RefreshTftPlayerMutationVariables>(RefreshTftPlayerDocument, variables)(),
    ...options
  }
    )};


useRefreshTftPlayerMutation.fetcher = (variables: Operations.RefreshTftPlayerMutationVariables, options?: RequestInit['headers']) => fetcher<Operations.RefreshTftPlayerMutation, Operations.RefreshTftPlayerMutationVariables>(RefreshTftPlayerDocument, variables, options);

export const TftLookupJobDocument = new TypedDocumentString(`
    query TFTLookupJob($id: ID!) {
  tftLookupJob(id: $id) {
    id
    platform
    gameName
    tagLine
    status
    stage
    scannedCount
    fetchedCount
    failedCount
    errorCode
    createdAt
    updatedAt
    completedAt
  }
}
    `);

export const useTftLookupJobQuery = <
      TData = Operations.TftLookupJobQuery,
      TError = unknown
    >(
      variables: Operations.TftLookupJobQueryVariables,
      options?: Omit<UseQueryOptions<Operations.TftLookupJobQuery, TError, TData>, 'queryKey'> & { queryKey?: UseQueryOptions<Operations.TftLookupJobQuery, TError, TData>['queryKey'] }
    ) => {

    return useQuery<Operations.TftLookupJobQuery, TError, TData>(
      {
    queryKey: ['TFTLookupJob', variables],
    queryFn: fetcher<Operations.TftLookupJobQuery, Operations.TftLookupJobQueryVariables>(TftLookupJobDocument, variables),
    ...options
  }
    )};

useTftLookupJobQuery.getKey = (variables: Operations.TftLookupJobQueryVariables) => ['TFTLookupJob', variables];


useTftLookupJobQuery.fetcher = (variables: Operations.TftLookupJobQueryVariables, options?: RequestInit['headers']) => fetcher<Operations.TftLookupJobQuery, Operations.TftLookupJobQueryVariables>(TftLookupJobDocument, variables, options);
