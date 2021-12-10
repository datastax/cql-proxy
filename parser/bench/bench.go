package main

import (
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/datastax/cql-proxy/parser"
)

var selectSystem, _ = regexp.Compile("^\\s*(?i:SELECT\\s.*\\sFROM\\s+\"?(?:system\\.|local|peers))|(?i:USE\\s+\"?[a-zA-Z0-9_]+\"?)")

var queries = []string{
	"\n   SELECT * FROM\nsystem.local",
	"\tSELECT key FROM system.local",
	"SELECT * FROM system.peers",
	"SELECT * FROM system.peers_v2",
	"SELECT * FROM local",
	"SELECT * FROM peers",
	"SELECT * FROM \"peers\"",
	"SELECT * FROM \"local\"",
	"SELECT * FROM \"system\".\"local\"",
	"SELECT a, b, c FROM peers",
	"SELECT event_id, \n  dateOf(created_at) AS creation_date,\n  blobAsText(content) AS content \n  FROM timeline;",
	"SELECT COUNT(*) \nFROM system.IndexInfo;",
	"SELECT lastname \nFROM cycling.cyclist_name \nLIMIT 50000;",
	"SELECT id, lastname, teams \nFROM cycling.cyclist_career_teams \nWHERE id=5b6962dd-3f90-4c93-8f61-eabfa4a803e2;",
	"SELECT sum(race_points) \nFROM cycling.cyclist_points \nWHERE id=e3b19ec4-774a-4d1c-9e5a-decec1e30aac \n      AND race_points > 7;",
	"SELECT first_name, last_name \nFROM emp \nWHERE empID IN (105, 107, 104);",
	"SELECT * \nFROM parts \nWHERE part_type='alloy' AND part_name='hubcap' \nAND part_num=1249 AND part_year IN ('2010', '2015');",
	"SELECT * \nFROM parts \nWHERE part_num=123456 AND part_year IN ('2010', '2015') \nALLOW FILTERING;",
	"SELECT album, tags \nFROM playlists \nWHERE tags CONTAINS 'blues';",
	"SELECT * \nFROM playlists \nWHERE venue \nCONTAINS 'The Fillmore';",
	"SELECT * \nFROM playlists \nWHERE venue CONTAINS KEY '2014-09-22 22:00:00-0700';",
	"SELECT * \nFROM cycling.birthday_list \nWHERE blist['age'] = '23';",
	"SELECT * \nFROM cycling.race_starts \nWHERE rnumbers = [39,7,14];",
	"SELECT * FROM ruling_stewards\nWHERE king = 'Brego'\n  AND reign_start >= 2450\n  AND reign_start < 2500 \nALLOW FILTERING;",
	"Select * \nFROM ruling_stewards\nWHERE king = 'none'\n  AND reign_start >= 1500\n  AND reign_start < 3000 \nLIMIT 10 \nALLOW FILTERING;",
	"SELECT * \nFROM ruling_stewards \nWHERE (steward_name, king) = ('Boromir', 'Brego');",
	"SELECT * FROM playlists \nWHERE id = 62c36092-82a1-3a00-93d1-46196ee77204\nORDER BY song_order DESC \nLIMIT 50;",
	"SELECT album, title \nFROM playlists \nWHERE artist = 'Fu Manchu';",
	"SELECT * \nFROM cycling.last_3_days \nWHERE TOKEN(year) < TOKEN('2015-05-26') \n  AND year IN ('2015-05-24','2015-05-25');",
	"SELECT COUNT(lastname) \nFROM cycling.cyclist_name;",
	"SELECT COUNT(*) \nFROM users;",
	"SELECT name, max(points), COUNT(*) \nFROM users;",
	"SELECT MAX(points) \nFROM cycling.cyclist_category;",
	"SELECT category, MIN(points) \nFROM cycling.cyclist_category \nWHERE category = 'GC';",
	"SELECT WRITETIME (first_name) \nFROM users \nWHERE last_name = 'Jones';",
	"INSERT INTO cycling.calendar (race_id, race_name, race_start_date, race_end_date) \nVALUES (200, 'placeholder', '2015-05-27', '2015-05-27') \nUSING TTL;",
	"UPDATE cycling.calendar \nUSING TTL 300 \nSET race_name = 'dummy' \nWHERE race_id = 200 \n  AND race_start_date = '2015-05-27' \n  AND race_end_date = '2015-05-27';",
	"SELECT TTL(race_name) \nFROM cycling.calendar \nWHERE race_id=200;",
	"DELETE firstname, lastname FROM cycling.cyclist_name \nWHERE id = e7ae5cf3-d358-4d99-b900-85902fda9bb0;",
	"DELETE FROM cycling.cyclist_name \nWHERE id=e7ae5cf3-d358-4d99-b900-85902fda9bb0 IF EXISTS;",
	"DELETE FROM cycling.cyclist_name \nWHERE id =e7ae5cf3-d358-4d99-b900-85902fda9bb0 \nif firstname='Alex' and lastname='Smith';",
	"DELETE id FROM cyclist_id \nWHERE lastname = 'WELTEN' and firstname = 'Bram' \nIF EXISTS;",
	"DELETE id FROM cyclist_id \nWHERE lastname = 'WELTEN' AND firstname = 'Bram' \nIF age = 2000;",
	"DELETE firstname, lastname\n  FROM cycling.cyclist_name\n  USING TIMESTAMP 1318452291034\n  WHERE lastname = 'VOS';",
	"DELETE FROM cycling.cyclist_name \nWHERE id = 6ab09bec-e68e-48d9-a5f8-97e6fb4c9b47;",
	"DELETE FROM cycling.cyclist_name \nWHERE firstname IN ('Alex', 'Marianne');",
	"DELETE sponsorship ['sponsor_name'] FROM cycling.races \nWHERE race_name = 'Criterium du Dauphine';",
	"DELETE categories[3] FROM cycling.cyclist_history \nWHERE lastname = 'TIRALONGO';",
	"DELETE sponsorship FROM cycling.races \nWHERE race_name = 'Criterium du Dauphine';",
	"UPDATE cycling.cyclist_name\nSET comments ='='Rides hard, gets along with others, a real winner'\nWHERE id = fb372533-eb95-4bb4-8685-6ef61e994caa IF EXISTS;",
	"UPDATE users\n  SET state = 'TX'\n  WHERE user_uuid\n  IN (88b8fd18-b1ed-4e96-bf79-4280797cba80,\n    06a8913c-c0d6-477c-937d-6c1b69a95d43,\n    bc108776-7cb5-477f-917d-869c12dfffa8);",
	"UPDATE cycling.cyclists\n  SET firstname = 'Marianne',\n  lastname = 'VOS'\n  WHERE id = 88b8fd18-b1ed-4e96-bf79-4280797cba80;",
	"UPDATE excelsior.clicks USING TTL 432000\n  SET user_name = 'bob'\n  WHERE userid=cfd66ccc-d857-4e90-b1e5-df98a3d40cd6 AND\n    url='http://google.com';",
	"UPDATE cycling.popular_count SET popularity = popularity + 2 WHERE id = 6ab09bec-e68e-48d9-a5f8-97e6fb4c9b47;",
	"UPDATE cycling.cyclists\nSET firstname = 'Anna', lastname = 'VAN DER BREGGEN' WHERE id = e7cd5752-bc0d-4157-a80f-7523add8dbcd;",
	"UPDATE cycling.upcoming_calendar \nSET events = ['Criterium du Dauphine','Tour de Suisse'];",
	"UPDATE cycling.upcoming_calendar \nSET events = ['Tour de France'] + events WHERE year=2015 AND month=06;",
	"UPDATE cycling.upcoming_calendar \nSET events[4] = 'Tour de France' WHERE year=2016 AND month=07;",
	"UPDATE cycling.upcoming_calendar \nSET events = events - ['Criterium du Dauphine'] WHERE year=2016 AND month=07;",
	"UPDATE cycling.cyclist_career_teams\nSET teams = teams + {'Team DSB - Ballast Nedam'} WHERE id=5b6962dd-3f90-4c93-8f61-eabfa4a803e2;",
	"UPDATE cycling.cyclist_career_teams\nSET teams = teams - {'DSB Bank Nederland bloeit'} WHERE id=5b6962dd-3f90-4c93-8f61-eabfa4a803e2;",
	"UPDATE cycling.cyclist_career_teams\nSET teams = {} WHERE id=5b6962dd-3f90-4c93-8f61-eabfa4a803e2;",
	"UPDATE cycling.upcoming_calendar \nSET description = description + {'Criterium du Dauphine' : 'Easy race'} WHERE year = 2015;\n",
	"UPDATE cycling.upcoming_calendar \nSET events[2] = 'Vuelta Ciclista a Venezuela' WHERE year = 2016 AND month = 06;",
	"UPDATE cycling.upcoming_calendar USING TTL <ttl_value>\nSET events[2] = 'Vuelta Ciclista a Venezuela' WHERE year = 2016 AND month = 06;",
	"UPDATE cycling.upcoming_calendar \nSET description = description + {'Criterium du Dauphine' : 'Easy race', 'Tour du Suisse' : 'Hard uphill race'}\nWHERE year = 2015 AND month = 6;",
	"UPDATE cycling.upcoming_calendar \nSET description = \n{'Criterium du Dauphine' : 'Easy race', \n 'Tour du Suisse' : 'Hard uphill race'} \nWHERE year = 2015 AND month = 6;",
	"UPDATE cycling.upcoming_calendar \nSET description = description + { 'Tour de France' : 'Very competitive'} \nWHERE year = 2015 AND month = 6;",
	"UPDATE cycling.upcoming_calendar \nSET description = \n{'Criterium du Dauphine' : 'Easy race', \n 'Tour du Suisse' : 'Hard uphill race',\n 'Tour de France' : 'Very competitive'} \nWHERE year = 2015 AND month = 6;",
	"UPDATE cycling.cyclist_id SET age = 28 WHERE lastname = 'WELTEN' and firstname = 'Bram' IF EXISTS;",
	"UPDATE cyclist_id SET id = 15a116fc-b833-4da6-ab9a-4a3775750239 where lastname = 'WELTEN' and firstname = 'Bram' IF age = 18;",
	"BEGIN BATCH\n     INSERT INTO mytable (a, b, d) values (7, 7, 'a')\n     UPDATE mytable SET s = 1 WHERE a = 1 IF s = NULL;\nAPPLY BATCH",
	"BEGIN BATCH\n     INSERT INTO mytable (a, b, d) values (7, 7, 'a')\n     UPDATE mytable SET s = 7 WHERE a = 7 IF s = NULL;\nAPPLY BATCH",
	"INSERT INTO cycling.cyclist_name (id, lastname, firstname)\n  VALUES (6ab09bec-e68e-48d9-a5f8-97e6fb4c9b47, 'KRUIKSWIJK','Steven')\n  USING TTL 86400 AND TIMESTAMP 123456789;",
	"INSERT INTO cycling.cyclist_categories (id,lastname,categories)\n  VALUES(\n    '6ab09bec-e68e-48d9-a5f8-97e6fb4c9b47', \n    'KRUIJSWIJK', \n    {'GC', 'Time-trial', 'Sprint'});",
	"INSERT INTO cycling.cyclist_teams (id,lastname,teams)\n  VALUES(\n    5b6962dd-3f90-4c93-8f61-eabfa4a803e2, \n    'VOS', \n    { 2015 : 'Rabobank-Liv Woman Cycling Team', \n      2014 : 'Rabobank-Liv Woman Cycling Team' });",
	"INSERT INTO cycling.cyclist_name (id, lastname, firstname) \n   VALUES (c4b65263-fe58-4846-83e8-f0e1c13d518f, 'RATTO', 'Rissella') \nIF NOT EXISTS; ",
	"USE PortfolioDemo;",
	"USE \"Excalibur\";",
	"BEGIN BATCH USING TIMESTAMP 1481124356754405\nINSERT INTO cycling.cyclist_expenses \n   (cyclist_name, expense_id, amount, description, paid) \n   VALUES ('Vera ADRIAN', 2, 13.44, 'Lunch', true);\nINSERT INTO cycling.cyclist_expenses \n   (cyclist_name, expense_id, amount, description, paid) \n   VALUES ('Vera ADRIAN', 3, 25.00, 'Dinner', true);\nAPPLY BATCH;",
	"SELECT cyclist_name, expense_id,\n        amount, WRITETIME(amount),\n        description, WRITETIME(description),\n        paid,WRITETIME(paid)\n   FROM cycling.cyclist_expenses\nWHERE cyclist_name = 'Vera ADRIAN';",
	"BEGIN BATCH\n  INSERT INTO purchases (user, balance) VALUES ('user1', -8) IF NOT EXISTS;\n  INSERT INTO purchases (user, expense_id, amount, description, paid)\n    VALUES ('user1', 1, 8, 'burrito', false);\nAPPLY BATCH;",
	"BEGIN BATCH\n  UPDATE purchases SET balance = -208 WHERE user='user1' IF balance = -8;\n  INSERT INTO purchases (user, expense_id, amount, description, paid)\n    VALUES ('user1', 2, 200, 'hotel room', false);\nAPPLY BATCH;",
	"BEGIN COUNTER BATCH\n  UPDATE UserActionCounts SET total = total + 2 WHERE keyalias = 523;\n  UPDATE AdminActionCounts SET total = total + 2 WHERE keyalias = 701;\nAPPLY BATCH;",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2015, 'Tour of Japan - Stage 4 - Minami > Shinshu', 'Benjamin PRADES', 1);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2015, 'Tour of Japan - Stage 4 - Minami > Shinshu', 'Adam PHELAN', 2);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2015, 'Tour of Japan - Stage 4 - Minami > Shinshu', 'Thomas LEBAS', 3);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2015, 'Giro d''Italia - Stage 11 - Forli > Imola', 'Ilnur ZAKARIN', 1);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2015, 'Giro d''Italia - Stage 11 - Forli > Imola', 'Carlos BETANCUR', 2);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank) \n   VALUES (2014, '4th Tour of Beijing', 'Phillippe GILBERT', 1);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank)  \n   VALUES (2014, '4th Tour of Beijing', 'Daniel MARTIN', 2);",
	"INSERT INTO cycling.rank_by_year_and_name (race_year, race_name, cyclist_name, rank)  \n   VALUES (2014, '4th Tour of Beijing', 'Johan Esteban CHAVES', 3);",
	"SELECT * FROM cycling.cyclist_category;",
	"SELECT * FROM cycling.cyclist_category WHERE category = 'SPRINT';",
	"SELECT category, points, lastname FROM cycling.cyclist_category;",
	"SELECT * From cycling.cyclist_name LIMIT 3;",
	"SELECT * FROM cycling.cyclist_cat_pts WHERE category = 'GC' ORDER BY points ASC;",
	"SELECT race_name, point_id, lat_long AS CITY_LATITUDE_LONGITUDE FROM cycling.route;",
	"select json name, checkin_id, timestamp from checkin;",
	"select name, checkin_id, toJson(timestamp) from checkin;",
	"INSERT INTO cycling.cyclist_category JSON '{\n  \"category\" : \"GC\", \n  \"points\" : 780, \n  \"id\" : \"829aa84a-4bba-411f-a4fb-38167a987cda\",\n  \"lastname\" : \"SUTHERLAND\" }';",
	"INSERT INTO cycling.cyclist_category JSON '{\n  \"category\" : \"Sprint\", \n  \"points\" : 700, \n  \"id\" : \"829aa84a-4bba-411f-a4fb-38167a987cda\"\n}';",
	"INSERT INTO cycling.cyclist_stats (id, lastname, basics) VALUES (\n  e7ae5cf3-d358-4d99-b900-85902fda9bb0, \n  'FRAME', \n  { birthday : '1993-06-18', nationality : 'New Zealand', weight : null, height : null }\n);",
	"INSERT INTO cycling.cyclist_races (id, lastname, firstname, races) VALUES (\n  5b6962dd-3f90-4c93-8f61-eabfa4a803e2,\n  'VOS',\n  'Marianne',\n  [{ race_title : 'Rabobank 7-Dorpenomloop Aalburg',race_date : '2015-05-09',race_time : '02:58:33' },\n  { race_title : 'Ronde van Gelderland',race_date : '2015-04-19',race_time : '03:22:23' }]\n);",
	"INSERT INTO cycling.route (race_id, race_name, point_id, lat_long) VALUES (500, '47th Tour du Pays de Vaud', 2, ('Champagne', (46.833, 6.65)));",
	"INSERT INTO cycling.nation_rank (nation, info) VALUES ('Spain', (1,'Alejandro VALVERDE' , 9054));",
	"INSERT INTO cycling.popular (rank, cinfo) VALUES (4, ('Italy', 'Fabio ARU', 163));",
	"INSERT INTO t (k, s, i) VALUES ('k', 'I''m shared', 0);",
	"INSERT INTO t (k, s, i) VALUES ('k', 'I''m still shared', 1);",
	"SELECT * FROM t;",
	"BEGIN BATCH\n  INSERT INTO cycling.cyclist_expenses (cyclist_name, balance) VALUES ('Vera ADRIAN', 0) IF NOT EXISTS;\n  INSERT INTO cycling.cyclist_expenses (cyclist_name, expense_id, amount, description, paid) VALUES ('Vera ADRIAN', 1, 7.95, 'Breakfast', false);\n  APPLY BATCH;",
	"UPDATE cycling.cyclist_expenses SET balance = -7.95 WHERE cyclist_name = 'Vera ADRIAN' IF balance = 0;",
}

var result int

func maybeSelectOrUse(query string) bool {
	for _, c := range query {
		switch c {
		case ' ', '\n', '\t':
			continue
		case 's', 'S':
			return true
		case 'u', 'U':
			return true
		default:
			return false
		}
	}
	return false
}

func BenchmarkSystemAndUseStatement_JustMaybeCheck(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			if maybeSelectOrUse(query) {
				r++
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_JustRegEx(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			if selectSystem.MatchString(query) {
				r++
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_ParserRagel(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			handled, _ := parser.IsQueryHandled(query)
			if handled {
				r++
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_RegEx(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			if selectSystem.MatchString(query) {
				handled, _, _ := parser.Parse("system", query)
				if handled {
					r++
				}
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_MaybeRegEx(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			if maybeSelectOrUse(query) && selectSystem.MatchString(query) {
				handled, _, _ := parser.Parse("system", query)
				if handled {
					r++
				}
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_Parser(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		r = 0
		for _, query := range queries {
			handled, _, _ := parser.Parse("system", query)
			if handled {
				r++
			}
		}
	}
	result = r
}

func BenchmarkSystemAndUseStatement_MaybeParser(b *testing.B) {
	var r int
	for n := 0; n < b.N; n++ {
		for _, query := range queries {
			if maybeSelectOrUse(query) {
				handled, _, _ := parser.Parse("system", query)
				if handled {
					r++
				}
			}
		}
	}
	result = r
}

func main() {
	numBytes := 0
	for _, q := range queries {
		numBytes += len(q)
	}

	mb := float64(1024 * 1024)

	var r testing.BenchmarkResult
	var seconds float64

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_RegEx)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("RegEx: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_MaybeRegEx)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("MaybeRegEx: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_Parser)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("ParserAntlr: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_MaybeParser)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("MaybeParser: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_ParserRagel)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("ParserRagel: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_JustMaybeCheck)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("JustMaybeCheck: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)

	r = testing.Benchmark(BenchmarkSystemAndUseStatement_JustRegEx)
	seconds = float64(r.T) / float64(time.Second) / float64(r.N)
	fmt.Printf("JustRegEx: %d ns/op %f mib/s\n", int(r.T)/r.N, float64(numBytes)/mb/seconds)
}
