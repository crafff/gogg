import type * as Operations from "./types";
import { DocumentTypeDecoration } from '@graphql-typed-document-node/core';
import { useQuery, UseQueryOptions } from '@tanstack/react-query';

function fetcher<TData, TVariables>(query: TypedDocumentString<unknown, unknown>, variables?: TVariables) {
  return async (): Promise<TData> => {
    const res = await fetch("/graphql" as string, {
    method: "POST",
    ...({"headers":{"Content-Type":"application/json"},"credentials":"same-origin"}),
      body: JSON.stringify({ query, variables }),
    });

    const json = await res.json();

    if (json.errors) {
      const { message } = json.errors[0];

      throw new Error(message);
    }

    return json.data;
  }
}

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


useVersionsQuery.fetcher = (variables?: Operations.VersionsQueryVariables) => fetcher<Operations.VersionsQuery, Operations.VersionsQueryVariables>(VersionsDocument, variables);

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


useRegionsQuery.fetcher = (variables?: Operations.RegionsQueryVariables) => fetcher<Operations.RegionsQuery, Operations.RegionsQueryVariables>(RegionsDocument, variables);

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


useChampionRankingsQuery.fetcher = (variables?: Operations.ChampionRankingsQueryVariables) => fetcher<Operations.ChampionRankingsQuery, Operations.ChampionRankingsQueryVariables>(ChampionRankingsDocument, variables);
