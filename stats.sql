SELECT count(*) FROM storj3_nodes;

SELECT params->>'type' AS type, count(*)
FROM storj3_nodes
GROUP BY params->>'type'
ORDER BY params->>'type';

SELECT dif, count(*), repeat('*', count(*)::int/4+1) AS count_chart
FROM (SELECT length(substring(('x'||encode(id, 'hex'))::bit(256)::text FROM '0*$')) AS dif
      FROM storj3_nodes WHERE params->>'type'='STORAGE') AS t
GROUP BY dif
ORDER BY dif;

SELECT delta AS hours, count(*), repeat('*', (count(*)/4+1)::int) AS count_chart, CASE WHEN delta<24 THEN '<<<' END AS today
FROM (SELECT (extract(epoch FROM now() - updated_at)/3600)::int AS delta, *
      FROM storj3_nodes) AS t
GROUP BY delta
ORDER BY delta;
